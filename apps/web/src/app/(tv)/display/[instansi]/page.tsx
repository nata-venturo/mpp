import type { Metadata } from 'next';

import { DisplayView } from 'src/sections/tv/view/display-view';

export const metadata: Metadata = { title: 'Display Antrean' };

type Props = { params: Promise<{ instansi: string }> };

export default async function Page({ params }: Props) {
  const { instansi } = await params;

  return <DisplayView instansiId={instansi} />;
}
