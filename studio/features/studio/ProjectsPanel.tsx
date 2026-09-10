'use client';
import { useState } from 'react';
import { Input } from '@/components/ui/input';
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
} from '@/components/ui/sheet';
import type { Project } from '../../lib/api/projects';

export default function ProjectsPanel({
  projects,
  busy,
  open,
  edit,
  refresh,
}: {
  projects: Project[];
  busy: boolean;
  open: (project: Project) => void;
  edit: (
    name: string,
    description: string,
    project?: Project,
  ) => Promise<boolean>;
  refresh: () => void;
}) {
  const [form, setForm] = useState<{
    project?: Project;
    name: string;
    description: string;
  } | null>(null);
  const [error, setError] = useState('');
  return (
    <>
      <div className="actions" style={{ margin: '0 30px 20px' }}>
        <button
          disabled={busy}
          onClick={() => {
            setError('');
            setForm({ name: '', description: '' });
          }}
        >
          프로젝트 생성
        </button>
        <button disabled={busy} onClick={refresh}>
          목록 새로고침
        </button>
      </div>
      <div className="projects-grid">
        {projects.map((project) => (
          <article key={project.id}>
            <span className="pill">LOCAL DB</span>
            <h2>{project.name}</h2>
            <p>{project.description}</p>
            <button disabled={busy} onClick={() => open(project)}>
              토폴로지 열기
            </button>
            <button
              disabled={busy}
              onClick={() => {
                setError('');
                setForm({
                  project,
                  name: project.name,
                  description: project.description,
                });
              }}
            >
              프로젝트 수정
            </button>
          </article>
        ))}
        {!projects.length && (
          <p>
            등록된 프로젝트가 없습니다. API 연결을 확인하고 프로젝트를
            생성하세요.
          </p>
        )}
      </div>
      <Sheet
        open={!!form}
        onOpenChange={(value) => {
          if (!value && !busy) setForm(null);
        }}
      >
        <SheetContent className="inspector">
          <SheetHeader>
            <SheetTitle>
              {form?.project ? '프로젝트 수정' : '프로젝트 생성'}
            </SheetTitle>
            <SheetDescription>
              로컬 Studio 데이터베이스에 저장합니다.
            </SheetDescription>
          </SheetHeader>
          {form && (
            <form
              className="inspector-fields"
              onSubmit={async (event) => {
                event.preventDefault();
                setError('');
                if (await edit(form.name, form.description, form.project))
                  setForm(null);
                else
                  setError(
                    '저장하지 못했습니다. 연결 상태를 확인하세요. 수정 충돌이면 닫고 목록을 새로고침하세요.',
                  );
              }}
            >
              <label className="field">
                <span>프로젝트 이름</span>
                <Input
                  required
                  maxLength={128}
                  value={form.name}
                  disabled={busy}
                  onChange={(e) => setForm({ ...form, name: e.target.value })}
                />
              </label>
              <label className="field">
                <span>설명</span>
                <Input
                  maxLength={2000}
                  value={form.description}
                  disabled={busy}
                  onChange={(e) =>
                    setForm({ ...form, description: e.target.value })
                  }
                />
              </label>
              <p role="alert">{error}</p>
              <button disabled={busy || !form.name.trim()} type="submit">
                저장
              </button>
            </form>
          )}
        </SheetContent>
      </Sheet>
    </>
  );
}
