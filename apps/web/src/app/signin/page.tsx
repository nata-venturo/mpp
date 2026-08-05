import type { Metadata } from 'next';

import { SignInView } from 'src/sections/auth/view/sign-in-view';

export const metadata: Metadata = { title: 'Masuk Petugas' };

export default function Page() {
  return <SignInView />;
}
