import type { Metadata } from 'next';

import { QueueStatusView } from 'src/sections/citizen/view/queue-status-view';

export const metadata: Metadata = { title: 'Status Antrean' };

export default function Page() {
  return <QueueStatusView />;
}
