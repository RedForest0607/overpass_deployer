'use client';
import { useState } from 'react';
import JSZip from 'jszip';
import { Download, FileCode, Package, Copy, Check } from 'lucide-react';
export function downloadBlob(blob: Blob, name: string) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = name;
  a.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
export default function PackagePreview({
  files,
  onBack,
}: {
  files: Record<string, string>;
  onBack: () => void;
}) {
  const [selected, setSelected] = useState(Object.keys(files)[0]);
  const [message, setMessage] = useState('');
  const [busy, setBusy] = useState(false);
  async function download() {
    setBusy(true);
    try {
      const zip = new JSZip();
      Object.entries(files).forEach(([name, content]) =>
        zip.file(name, content),
      );
      downloadBlob(
        await zip.generateAsync({ type: 'blob' }),
        'overpass-design-draft.zip',
      );
      setMessage('설계 초안을 다운로드했습니다.');
    } catch {
      setMessage('ZIP 생성에 실패했습니다. 다시 시도하세요.');
    } finally {
      setBusy(false);
    }
  }
  async function copy() {
    try {
      await navigator.clipboard.writeText(files[selected]);
      setMessage('클립보드에 복사했습니다.');
    } catch {
      setMessage('복사 권한이 없습니다. ZIP 다운로드를 이용하세요.');
    }
  }
  return (
    <section className="package-view">
      <div className="package-heading">
        <div>
          <span className="eyebrow">GENERATED DESIGN</span>
          <h2>
            <Package /> 설계 산출물
          </h2>
        </div>
        <div className="actions">
          <button onClick={onBack}>편집기로 돌아가기</button>
          <button className="primary" disabled={busy} onClick={download}>
            <Download size={16} />
            {busy ? '생성 중…' : '초안 ZIP 다운로드'}
          </button>
        </div>
      </div>
      <div className="draft-notice">
        설계 초안 · 실제 JAR, 설정 파일, Secret은 포함되지 않습니다. Helm /
        Terraform 생성과 실제 배포는 아직 연결되지 않았습니다.
      </div>
      <div className="file-browser">
        <aside>
          {Object.keys(files).map((name) => (
            <button
              key={name}
              className={selected === name ? 'selected' : ''}
              onClick={() => {
                setSelected(name);
                setMessage('');
              }}
            >
              <FileCode size={16} />
              {name}
            </button>
          ))}
        </aside>
        <div className="code-pane">
          <div className="code-header">
            <span>{selected}</span>
            <button onClick={copy}>
              <Copy size={14} /> 복사
            </button>
          </div>
          <pre>{files[selected]}</pre>
        </div>
      </div>
      <p role="status" className="feedback">
        <Check size={15} />
        {message || '현재 토폴로지에서 생성된 설계 스냅샷입니다.'}
      </p>
    </section>
  );
}
