import type { Metadata } from 'next';

import { BookingView } from 'src/sections/citizen/view/booking-view';

export const metadata: Metadata = { title: 'Daftar Layanan' };

export default function Page() {
  return <BookingView />;
}
