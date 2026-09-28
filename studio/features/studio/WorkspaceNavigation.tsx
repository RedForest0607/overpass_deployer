import {
  Folder,
  FileArchive,
  Workflow,
  Package,
  ChevronRight,
} from 'lucide-react';
import {
  SidebarMenu,
  SidebarMenuItem,
  SidebarMenuButton,
} from '@/components/ui/sidebar';

export default function WorkspaceNavigation({
  view,
  project,
  environment,
  onNavigate,
}: {
  view: string;
  project?: string;
  environment: string;
  onNavigate: (view: string) => void;
}) {
  return (
    <nav aria-label="작업공간 메뉴" className="workspace-navigation">
      <small>WORKSPACE</small>
      <SidebarMenu>
        {[
          { id: 'projects', label: '프로젝트', Icon: Folder },
          { id: 'artifacts', label: '파일', Icon: FileArchive },
        ].map(({ id, label, Icon }) => (
          <SidebarMenuItem key={id}>
            <SidebarMenuButton
              className="nav-item"
              isActive={view === id}
              aria-current={view === id ? 'page' : undefined}
              onClick={() => onNavigate(id)}
            >
              <Icon size={18} />
              <span>{label}</span>
            </SidebarMenuButton>
          </SidebarMenuItem>
        ))}
      </SidebarMenu>
      <div className="project-navigation">
        <small>CURRENT PROJECT</small>
        <button
          className="current-project-link"
          disabled={!project}
          onClick={() => onNavigate('topology')}
          title={project}
        >
          <span className="project-dot" />
          <span>{project ?? '프로젝트를 선택하세요'}</span>
          <ChevronRight size={14} />
        </button>
        {project && <span className="secondary">{environment}</span>}
        <SidebarMenu className="project-submenu">
          {[
            { id: 'topology', label: '토폴로지', Icon: Workflow },
            { id: 'packages', label: '배포 패키지', Icon: Package },
          ].map(({ id, label, Icon }) => (
            <SidebarMenuItem key={id}>
              <SidebarMenuButton
                className="nav-item"
                disabled={!project}
                isActive={view === id}
                aria-current={view === id ? 'page' : undefined}
                onClick={() => onNavigate(id)}
              >
                <Icon size={18} />
                <span>{label}</span>
              </SidebarMenuButton>
            </SidebarMenuItem>
          ))}
        </SidebarMenu>
      </div>
    </nav>
  );
}
