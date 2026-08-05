import type { Metadata } from 'next';

import { KioskIdleView } from 'src/sections/kiosk/view/kiosk-idle-view';

export const metadata: Metadata = { title: 'Kiosk MPP' };

export default function Page() {
  return <KioskIdleView />;
}
