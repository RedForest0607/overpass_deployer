import { Plus, CircleHelp } from 'lucide-react';
import { catalog, type Kind } from '../topology/model';
import { ResourceIcon } from '../topology/ResourceNode';
import DeploymentProfileEditor from '../topology/DeploymentProfileEditor';
import type { DeploymentProfile } from '../../lib/api/contracts';

export default function TopologyPalette({
  paletteOpen,
  deployment,
  setDeployment,
  add,
}: {
  paletteOpen: boolean;
  deployment: DeploymentProfile;
  setDeployment: (value: DeploymentProfile) => void;
  add: (kind: Kind) => void;
}) {
  return (
    <aside className={'palette ' + (paletteOpen ? 'open' : '')}>
      <h3>
        노드 라이브러리 <span>{catalog.length}</span>
      </h3>
      <p className="palette-help">클릭 또는 캔버스로 드래그</p>
      {Array.from(new Set(catalog.map((c) => c.group))).map((group) => (
        <section key={group}>
          <small>{group}</small>
          {catalog
            .filter((c) => c.group === group)
            .map((c) => (
              <button
                key={c.kind}
                className="palette-node"
                draggable
                onDragStart={(e) => {
                  e.dataTransfer.setData('application/overpass-node', c.kind);
                  e.dataTransfer.effectAllowed = 'move';
                }}
                onClick={() => add(c.kind)}
              >
                <span style={{ color: c.color }}>
                  <ResourceIcon kind={c.kind} size={18} />
                </span>
                <span>
                  <strong>{c.title}</strong>
                  <small>{c.subtitle}</small>
                </span>
                <Plus size={13} />
              </button>
            ))}
        </section>
      ))}
      <div className="palette-note">
        <CircleHelp size={16} />
        <span>
          Java·Redis·Elasticsearch는 VM 내부로 드래그하세요. 실행 대상은
          설정에서 변경하며, 핸들은 통신 연결에 사용합니다.
        </span>
      </div>
      <DeploymentProfileEditor value={deployment} onChange={setDeployment} />
    </aside>
  );
}
