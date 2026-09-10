import { stringify } from 'yaml';
import type { TopologyNode, TopologyEdge } from '../topology/model';
export function filesFor(
  nodes: TopologyNode[],
  edges: TopologyEdge[],
  environment: string,
) {
  const blueprint = {
    apiVersion: 'studio.overpass.io/draft',
    environment,
    nodes: nodes.map(({ id, data, position, parentId }) => ({
      id,
      ...data,
      position,
      parentId,
    })),
    edges: edges.map(({ id, source, target, data }) => ({
      id,
      source,
      target,
      ...data,
    })),
  };
  const servers = nodes
    .filter((n) => n.data.kind === 'vm')
    .map((vm) => ({
      name: vm.data.label,
      host: vm.data.host,
      tags: [environment.toLowerCase()],
      apps: edges
        .filter((e) => e.target === vm.id)
        .flatMap((e) => {
          const app = nodes.find(
            (n) => n.id === e.source && n.data.kind === 'java',
          );
          if (!app) return [];
          const base = '/app/overpass/' + app.data.label;
          return [
            {
              name: app.data.label,
              base_dir: base,
              port: app.data.port,
              jar: {
                local_path: './assets/' + (app.data.artifact || 'MISSING.jar'),
                remote_path: base + '/lib/app.jar',
              },
              jvm: { min_heap: '256m', max_heap: app.data.heap },
              script: {
                mode: 'template',
                template: 'embedded:server.sh.tmpl',
                remote_path: base + '/bin/server.sh',
              },
            },
          ];
        }),
    }));
  const note =
    '# 설계 초안: JAR/설정/SSH 키가 포함되지 않습니다.\n# 패키지 디렉터리에서 기존 deploy vm 검증을 수행한 뒤 사용하세요.\n# DB/Redis 연결과 방화벽 규칙은 이 VM 초안에 반영되지 않습니다.\n';
  const result: Record<string, string> = {
    'topology.json': JSON.stringify(blueprint, null, 2),
    'README.txt':
      'Overpass Studio — Frontend draft\n\n설계 데이터와 VM YAML 초안입니다. 실행 가능한 배포 패키지가 아닙니다.\nJAR, application.yml, env, SSH 키는 포함되지 않습니다.\nRedis/Elasticsearch 설치·실행 설정은 생성하지 않습니다.\nDB/Redis 바인딩, Helm, Terraform 생성은 백엔드 연동 후 지원 예정입니다.\n실제 배포 전 별도 검증이 필요합니다.\n',
  };
  if (servers.length)
    result['deploy.draft.yml'] =
      note +
      stringify({
        ssh: {
          user: 'deploy',
          key_path: '${DEPLOY_SSH_KEY_PATH}',
          host_key_checking: 'strict',
          known_hosts_path: '${DEPLOY_KNOWN_HOSTS}',
        },
        servers,
      });
  return result;
}
