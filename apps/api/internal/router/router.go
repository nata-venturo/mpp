package router

import (
	"context"
	"time"
	"github.com/ndollem/mpp/apps/api/internal/config"
	"github.com/ndollem/mpp/apps/api/internal/middleware"

	// Core modules
	"github.com/ndollem/mpp/apps/api/internal/modules/core/api_key"
	"github.com/ndollem/mpp/apps/api/internal/modules/core/approval"
	"github.com/ndollem/mpp/apps/api/internal/modules/core/auth"
	"github.com/ndollem/mpp/apps/api/internal/modules/core/branch"
	"github.com/ndollem/mpp/apps/api/internal/modules/core/client"
	"github.com/ndollem/mpp/apps/api/internal/modules/core/company"
	"github.com/ndollem/mpp/apps/api/internal/modules/core/role"
	"github.com/ndollem/mpp/apps/api/internal/modules/core/translation_overrides"
	"github.com/ndollem/mpp/apps/api/internal/modules/core/user"
	userRepo "github.com/ndollem/mpp/apps/api/internal/modules/core/user/repository"

	// MPP modules
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/antrian"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/booking"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/checkin"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/display"
	mppconfig "github.com/ndollem/mpp/apps/api/internal/modules/mpp/config"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/instansi"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/kuota"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/loket"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/loket_ops"
	"github.com/ndollem/mpp/apps/api/internal/modules/mpp/ws"

	"github.com/ndollem/mpp/apps/api/internal/shared/audit"
	"github.com/ndollem/mpp/apps/api/internal/shared/authz"
	sharedRedis "github.com/ndollem/mpp/apps/api/internal/shared/redis"

	pkgfirebase "github.com/ndollem/mpp/apps/api/pkg/firebase"
	"github.com/ndollem/mpp/apps/api/pkg/logger"

	"github.com/gin-contrib/cors"
	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

func Setup(router *gin.Engine, db *pgxpool.Pool, cfg *config.Config) {
	// Get logger instance
	log := logger.GetLogger()

	// ─── Redis & authz cache ────────────────────────────────────────
	// Redis backs the per-user permission cache (see
	// internal/shared/authz). It is load-bearing: RequirePermission
	// middleware refuses to answer without it, so a failed bootstrap
	// here is fatal rather than a silent degrade.
	redisClient, err := sharedRedis.New(context.Background(), cfg.Redis)
	if err != nil {
		log.Fatal("Failed to connect to Redis", zap.Error(err))
	}
	log.Info("Redis connected",
		zap.String("addr", cfg.Redis.Host+":"+cfg.Redis.Port),
		zap.Int("db", cfg.Redis.DB),
		zap.Duration("permission_ttl", cfg.Redis.PermissionTTL),
	)

	// Ginzap middleware for logging HTTP requests
	router.Use(ginzap.Ginzap(log, time.RFC3339, true))

	// Recovery middleware with Zap (handles panics)
	router.Use(ginzap.RecoveryWithZap(log, true))

	// CORS middleware configuration
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173", "http://localhost:3000", "http://localhost:3001", "http://localhost:8081", "https://app.tuai.id", "https://jesuit.venturo.pro", "https://skeleton.venturo.id"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-API-Key"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"message": "Tuai API is running",
		})
	})

	// Core v1 routes
	coreV1 := router.Group("/core/v1")
	{
		// Initialize and setup client module. Must come before auth so
		// that SignUp can provision a client for new registrants.
		clientModule := client.Initialize(db)
		clientModule.SetupRoutes(coreV1)

		// Initialize and setup auth module
		authModule := auth.Initialize(db, cfg)
		authModule.SetupRoutes(coreV1)

		// Wire the user_identities repo (used by Google sign-in to look
		// up / link Firebase identities to a core.users row).
		authModule.Service.SetUserIdentityRepo(userRepo.NewUserIdentityRepository(db))

		// Wire the Firebase Admin client when configured. If the
		// operator hasn't set FIREBASE_PROJECT_ID the verifier stays
		// nil; the /auth/google endpoint will then surface a 503
		// "Google sign-in is not configured" instead of crashing.
		if cfg.Firebase.ProjectID != "" {
			fbClient, err := pkgfirebase.New(context.Background(), cfg.Firebase.ProjectID, cfg.Firebase.CredentialsJSON)
			if err != nil {
				log.Fatal("Failed to initialize Firebase Admin", zap.Error(err))
			}
			authModule.Service.SetFirebaseVerifier(fbClient)
			log.Info("Firebase Admin initialized", zap.String("project_id", cfg.Firebase.ProjectID))
		} else {
			log.Warn("FIREBASE_PROJECT_ID not set — /auth/google endpoint disabled")
		}

		// Initialize and setup user module
		userModule := user.Initialize(db)
		userModule.SetupRoutes(coreV1)

		// Initialize and setup role module
		roleModule := role.Initialize(db)
		roleModule.SetupRoutes(coreV1)

		// Wire the permission cache now that both Redis and the role repo
		// exist. The role repo acts as the authoritative fetcher on cache
		// miss; role service uses the same cache to invalidate stale
		// entries when role permissions or assignments change; auth service
		// uses it to serve /auth/me without hitting the DB on every call.
		authzService := authz.NewService(redisClient, roleModule.Repository, cfg.Redis.PermissionTTL)
		middleware.SetAuthzService(authzService)
		roleModule.Service.SetPermissionCacheInvalidator(authzService)
		authModule.Service.SetPermissionReader(authzService)

		// Initialize and setup company module
		companyModule := company.Initialize(db)
		companyModule.SetupRoutes(coreV1)
		companyModule.SetupUserCompanyRoutes(coreV1)

		// Initialize and setup branch module
		branchModule := branch.Initialize(db)
		branchModule.SetupRoutes(coreV1)
		branchModule.SetupUserBranchRoutes(coreV1)

		// Wire up auth service dependencies for company operations
		authModule.Service.SetCompanyUserRepo(companyModule.UserRepository)
		authModule.Service.SetCompanyRepo(companyModule.Repository)
		authModule.Service.SetRoleRepo(roleModule.Repository)
		authModule.Service.SetBranchRepo(branchModule.Repository)
		authModule.Service.SetClientService(clientModule.Service)

		// Wire the client lookup into company service so Create can
		// resolve client_id for owners who have no primary company yet.
		companyModule.Service.SetClientLookup(clientModule.Repository)

		// Wire branch repo into company service so Create seeds a default branch
		companyModule.Service.SetBranchRepo(branchModule.Repository)

		// Wire default admin role so the creator gets full company-scoped
		// permissions out of the box (mirrors SignUp).
		companyModule.Service.SetDefaultAdminRoleID(cfg.Auth.DefaultAdminRoleID)

		// Wire the company-scope resolver into the user-branch service so
		// non-super-admin callers can only assign branches inside their
		// company subtree.
		branchModule.UserBranchService.SetScopeResolver(companyModule.Repository)

		// Wire the company membership lookup into the branch service so
		// /branches/by-companies can resolve non-super_admin callers'
		// allowed companies from core.company_users.
		branchModule.Service.SetCompanyMembershipLookup(companyModule.UserRepository)

		// Wire up user service dependencies for company sync on create
		userModule.Service.SetCompanySyncer(companyModule.Service)
		userModule.Service.SetBranchSyncer(branchModule.UserBranchService)
		userModule.Service.SetRoleLookup(roleModule.Repository)

		// Wire the live company-context verifier so that any endpoint guarded
		// by middleware.CompanyContext rejects stale JWT company claims (e.g.
		// user removed from the company, company soft-deleted/deactivated).
		middleware.SetCompanyContextVerifier(companyModule.UserRepository)

		// User module routes only run JWTAuth (not CompanyContext) because
		// super_admin paths must remain accessible without a tenant. Inject
		// the verifier directly into the user handler so its inline tenant
		// scope check enforces the same staleness guarantees.
		userModule.Handler.SetCompanyVerifier(companyModule.UserRepository)

		// Wire the branch-scope resolver so middleware.BranchScope() can
		// narrow branches reads to the caller's user_branches set.
		middleware.SetUserBranchResolver(branchModule.UserBranchRepository)

		// Initialize and setup API key module
		apiKeyModule := api_key.Initialize(db, roleModule.Repository)
		apiKeyModule.SetupRoutes(coreV1)

		// Wire up API key validator for middleware
		middleware.SetApiKeyValidator(apiKeyModule.Service)

		// Initialize and setup Approval module (depends on role repo for
		// snapshotting role names at submission time).
		approvalModule := approval.Initialize(db, roleModule.Repository)
		approvalModule.SetupRoutes(coreV1)

		// Initialize and setup Translation Overrides module. Exposes both
		// an unauthenticated bootstrap endpoint (resolves client by slug)
		// and super_admin CRUD nested under /admin/clients/:client_id.
		translationOverridesModule := translation_overrides.Initialize(db, clientModule.Service)
		translationOverridesModule.SetupRoutes(coreV1)
	}

	// MPP v1 routes — the queue domain. Public catalog/registration reads
	// carry no JWTAuth(); staff and device routes add JWTAuth() +
	// RequirePermission (the same middleware handles X-API-Key devices).
	mppV1 := router.Group("/mpp/v1")
	{
		instansiModule := instansi.Initialize(db, cfg.MPP.CompanyID)
		instansiModule.SetupRoutes(mppV1)

		loketModule := loket.Initialize(db, cfg.MPP.CompanyID)
		loketModule.SetupRoutes(mppV1)

		// Config has no routes of its own — it is the system_config reader
		// the queue engine consults (number format, QR window, TTS text).
		mppConfigModule := mppconfig.Initialize(db, cfg.MPP.Location)

		kuotaModule := kuota.Initialize(db, cfg.MPP.CompanyID, cfg.MPP.Location)
		kuotaModule.SetupRoutes(mppV1)

		bookingModule := booking.Initialize(db, cfg.MPP.CompanyID, cfg.MPP.Location,
			instansiModule.Repository, kuotaModule.Repository, mppConfigModule.Service)
		bookingModule.SetupRoutes(mppV1)

		antrianModule := antrian.Initialize(db, redisClient, cfg.MPP.CompanyID, cfg.MPP.Location,
			instansiModule.Repository, loketModule.Repository, bookingModule.Repository,
			mppConfigModule.Service)
		antrianModule.SetupRoutes(mppV1)

		// Check-in owns no repository: it drives the booking repo (token
		// lookup + status guard) and the antrian service (numbering) in
		// one transaction.
		checkinModule := checkin.Initialize(bookingModule.Repository, antrianModule.Service, cfg.MPP.Location)
		checkinModule.SetupRoutes(mppV1)

		// Realtime hub. Started before the modules that publish to it so
		// no transition is issued into a nil hub.
		wsModule := ws.Initialize(context.Background(), redisClient)
		wsModule.SetupRoutes(mppV1)

		loketOpsModule := loket_ops.Initialize(db, cfg.MPP.CompanyID,
			loketModule.Repository, antrianModule.Repository, instansiModule.Repository,
			antrianModule.Service, mppConfigModule.Service, wsModule.Hub)
		loketOpsModule.SetupRoutes(mppV1)

		displayModule := display.Initialize(db, instansiModule.Repository,
			antrianModule.Service, mppConfigModule.Service)
		displayModule.SetupRoutes(mppV1)

		// The hub needs the display module to answer a subscribe with a
		// snapshot; the display module needed the hub's channels first.
		wsModule.Hub.SetSnapshotProvider(displayModule.Service)
	}

	// Shared audit module (polymorphic, used by any feature that wants
	// per-entity history). Initialized after the core group so its routes
	// mount under /core/v1.
	auditModule := audit.Initialize(db)
	auditModule.SetupRoutes(coreV1)

	log.Info("Routes setup completed", zap.Int("routes", len(router.Routes())))
}
