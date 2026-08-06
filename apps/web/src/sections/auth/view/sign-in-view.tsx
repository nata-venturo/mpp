'use client';

import { z } from 'zod';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';

import Card from '@mui/material/Card';
import Stack from '@mui/material/Stack';
import Alert from '@mui/material/Alert';
import Button from '@mui/material/Button';
import Container from '@mui/material/Container';
import Typography from '@mui/material/Typography';

import { paths } from 'src/routes/paths';
import { useRouter, useSearchParams } from 'src/routes/hooks';

import { useSignInMutation } from 'src/lib/api/use-auth';

import { Form, Field } from 'src/components/hook-form';

// ----------------------------------------------------------------------

const SignInSchema = z.object({
  login: z.string().min(1, { message: 'Email atau username wajib diisi' }),
  password: z.string().min(1, { message: 'Kata sandi wajib diisi' }),
});

type SignInValues = z.infer<typeof SignInSchema>;

export function SignInView() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const signInMutation = useSignInMutation();

  // Setelah login, kembali ke layar yang tadi diminta (mis. /loket).
  const redirectTo = searchParams.get('next') || paths.mpp.loket;

  const methods = useForm<SignInValues>({
    resolver: zodResolver(SignInSchema),
    defaultValues: { login: '', password: '' },
  });

  const onSubmit = methods.handleSubmit(async (values) => {
    await signInMutation.mutateAsync(values);
    router.replace(redirectTo);
  });

  return (
    <Container maxWidth="sm" sx={{ py: 8 }}>
      <Typography variant="h3" sx={{ mb: 1 }}>
        Masuk petugas
      </Typography>
      <Typography variant="body2" sx={{ mb: 4, color: 'text.secondary' }}>
        Untuk petugas loket, front office, supervisor, dan admin MPP.
      </Typography>

      <Card sx={{ p: 3 }}>
        <Form methods={methods} onSubmit={onSubmit}>
          <Stack spacing={2.5}>
            {signInMutation.isError && (
              <Alert severity="error">
                {(signInMutation.error as Error).message || 'Gagal masuk. Periksa kredensial Anda.'}
              </Alert>
            )}

            <Field.Text name="login" label="Email atau username" autoComplete="username" />
            <Field.Text
              name="password"
              label="Kata sandi"
              type="password"
              autoComplete="current-password"
            />

            <Button
              type="submit"
              size="large"
              variant="contained"
              disabled={methods.formState.isSubmitting}
            >
              {methods.formState.isSubmitting ? 'Memproses…' : 'Masuk'}
            </Button>
          </Stack>
        </Form>
      </Card>
    </Container>
  );
}
