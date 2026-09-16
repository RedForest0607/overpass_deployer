# Studio에서 Redis와 Elasticsearch 배포 구성

Studio의 `crosswalk-vm` 생성기는 VM에 배치된 `hosted` Redis/Elasticsearch 노드를
Crosswalk `servers[].utils[]` 항목으로 변환한다. Crosswalk가 제품을 자동 설치하는 것은 아니다.
등록한 설정 파일·서비스 unit·압축 파일을 전달하고, 디렉터리를 만들고, 압축을 해제하고,
systemd 서비스의 enable/state를 맞추는 방식이다. 외부 관리형 Redis/Elasticsearch는
`location: external`로 연결 정보만 모델링하며 VM 배포 패키지에는 포함하지 않는다.

iSTOCKMALL DEV 전체 예시는 `studio/examples/istockmall-dev-topology.json`에 있다. 이 파일의
`artifact-replace-*`와 `secret-replace-*` 값은 설명용 ID이므로 Studio에 파일을 등록한 뒤 실제 ID로
교체해야 package 검증과 생성에 사용할 수 있다.

## iSTOCKMALL 자산 대응

현재 저장소에서 참고할 수 있는 대표 자산은 다음과 같다.

- Redis 설정: `ops/stock_company/dev/redis/redis.conf`
- Redis Sentinel 설정: `ops/stock_company/dev/redis/sentinel.conf`
- Redis Compose: `ops/stock_company/dev/redis/docker-compose.yml`
- Redis systemd unit: `ops/stock_company/dev/redis/istock-redis.service`
- Elasticsearch 압축 파일: `ops/stock_company/prod/extra_files/elasticsearch.tar.gz`
- Crosswalk utility 예제: `crosswalk/deploy.example.yml`

`ops/stock_company/current/*/deploy.yml`은 현황 기록 성격이므로 그대로 실행 입력으로 쓰지 말고,
필요한 경로·서비스명·사용자만 확인해서 Studio deployment 설정으로 옮긴다. 운영 자산은 업로드 전에
환경별 주소와 비밀 값이 들어 있지 않은지 반드시 확인한다. 특히 현재 DEV Redis/Sentinel/Compose
샘플에는 비밀번호와 고정 IP가 있으므로 원본을 다른 환경에 그대로 업로드하면 안 된다.

## 구성 순서

1. Studio의 Artifacts에서 설정/unit은 `file`, 압축 파일은 `archive`로 등록한다.
2. Redis 또는 Elasticsearch 노드를 만들고 `location`을 `hosted`로 둔다.
3. 노드를 실행할 VM 안에 배치한다. Crosswalk 생성기는 `process` placement만 지원한다.
4. 노드 Inspector의 Crosswalk deployment JSON에 디렉터리·파일·서비스를 입력한다.
5. 설계를 저장하고 package 검증 후 ZIP을 생성한다.
6. ZIP을 안전한 작업 디렉터리에서 풀고 Crosswalk의 검증/dry-run을 거친 뒤 배포한다.

Redis 예시:

```json
{
  "directories": ["/app/redis", "/app/redis/data"],
  "files": [
    {
      "artifactRef": "art_redis_conf",
      "packagePath": "files/redis/redis.conf",
      "remotePath": "/app/redis/redis.conf",
      "chmod": "0644"
    },
    {
      "artifactRef": "art_redis_sentinel",
      "remotePath": "/app/redis/sentinel/sentinel.conf",
      "chmod": "0644"
    },
    {
      "artifactRef": "art_redis_compose",
      "remotePath": "/app/redis/docker-compose.yml",
      "chmod": "0644"
    }
  ],
  "services": [{"name": "istock-redis", "enabled": true, "state": "restarted"}]
}
```

이 Redis 예시는 대상 VM에 Docker/Compose와 `istock-redis.service`가 이미 설치되어 있다는 전제다.
새 VM이라면 VM bootstrap에서 Docker/Compose를 준비하고, unit 파일은 권한 있는 provisioning 단계에서
설치하거나 `/etc/systemd/system` 상대 구조를 담은 별도 archive를 만들어 `sudo` 압축 해제로 전달한다.
일반 file delivery를 `/etc`에 직접 지정하면 SSH 사용자의 쓰기 권한 때문에 실패할 수 있다.

Elasticsearch 예시:

```json
{
  "directories": ["/app/elasticsearch"],
  "files": [
    {
      "artifactRef": "art_elasticsearch_archive",
      "packagePath": "files/elasticsearch/elasticsearch.tar.gz",
      "remotePath": "/tmp/elasticsearch.tar.gz",
      "extract": {
        "enabled": true,
        "remoteDir": "/app/elasticsearch",
        "stripComponents": 1,
        "sudo": true,
        "owner": "elasticsearch",
        "group": "elasticsearch",
        "chownRecursive": true
      }
    }
  ],
  "services": []
}
```

저장소의 Elasticsearch archive는 실행 스크립트를 포함하지만 systemd unit은 포함하지 않는다.
따라서 먼저 archive의 사용자·JDK·설정·실행 스크립트를 검토하고, 서비스로 관리하려면 별도로 검증한
unit을 설치한 후 `services`에 해당 이름을 선언한다. 노드 이름이 Crosswalk
utility 이름이 되므로 같은 VM의 수동 utility와 이름이 겹치면 생성이 거부된다. hosted 노드는
디렉터리·파일·서비스 중 적어도 하나를 선언해야 한다.
