import type { Metadata } from 'next';

import { BookingConfirmView } from 'src/sections/citizen/view/booking-confirm-view';

export const metadata: Metadata = { title: 'Konfirmasi Booking' };

type Props = { params: Promise<{ id: string }> };

export default async function Page({ params }: Props) {
  const { id } = await params;

  return <BookingConfirmView id={id} />;
}
