import { Workflow, Plus, Undo2, FileUp, Download } from 'lucide-react';

export default function TopologyToolbar({
  nodeCount,
  connectionCount,
  paletteOpen,
  setPaletteOpen,
  canUndo,
  undo,
  canImport,
  openImport,
  downloadJSON,
}: {
  nodeCount: number;
  connectionCount: number;
  paletteOpen: boolean;
  setPaletteOpen: (value: boolean) => void;
  canUndo: boolean;
  undo: () => void;
  canImport: boolean;
  openImport: () => void;
  downloadJSON: () => void;
}) {
  return (
    <div className="editor-toolbar">
      <div>
        <Workflow size={16} />
        <strong>Topology editor</strong>
        <span className="toolbar-divider" />
        <span>{nodeCount} nodes</span>
        <span>{connectionCount} connections</span>
      </div>
      <div>
        <button
          className="mobile-palette"
          onClick={() => setPaletteOpen(!paletteOpen)}
        >
          <Plus size={14} /> 노드
        </button>
        <button disabled={!canUndo} onClick={undo}>
          <Undo2 size={15} /> 삭제 취소
        </button>
        <button
          disabled={!canImport}
          title="토폴로지 JSON 가져오기"
          onClick={openImport}
        >
          <FileUp size={15} /> JSON 가져오기
        </button>
        <button title="토폴로지 JSON 다운로드" onClick={downloadJSON}>
          <Download size={15} /> JSON
        </button>
      </div>
    </div>
  );
}
