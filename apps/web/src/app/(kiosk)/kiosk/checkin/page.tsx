import type { Metadata } from 'next';

import { KioskCheckInView } from 'src/sections/kiosk/view/kiosk-checkin-view';

export const metadata: Metadata = { title: 'Check-in Kiosk' };

export default function Page() {
  return <KioskCheckInView />;
}
