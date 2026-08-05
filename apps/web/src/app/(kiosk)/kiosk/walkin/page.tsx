import type { Metadata } from 'next';

import { KioskWalkInView } from 'src/sections/kiosk/view/kiosk-walkin-view';

export const metadata: Metadata = { title: 'Daftar Walk-in' };

export default function Page() {
  return <KioskWalkInView />;
}
