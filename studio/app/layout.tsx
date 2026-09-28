import type { Metadata } from 'next';
import './globals.css';
import './workspace-theme.css';
export const metadata: Metadata = {
  title: 'Overpass Studio · Topology',
  description: '사내 배포 토폴로지 작업공간',
};
export default function Layout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="ko">
      <body>{children}</body>
    </html>
  );
}
