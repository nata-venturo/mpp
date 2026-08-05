import type { Metadata } from 'next';

import { LoketPanelView } from 'src/sections/loket/view/loket-panel-view';

export const metadata: Metadata = { title: 'Panel Loket' };

export default function Page() {
  return <LoketPanelView />;
}
