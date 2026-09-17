# Crosswalk 차트 호환 계약

## 포맷 식별자

새 차트의 최상위 필드는 다음과 같다.

```yaml
format: crosswalk/v1
```

`crosswalk/v1`은 현재 지원하는 정식 포맷이다. 기존 운영 차트와의 호환성을 위해 `format`이 없는 문서는 `crosswalk/legacy`로 정규화해 계속 읽고, 정규화된 설정을 다시 검증할 수 있도록 명시된 legacy 값도 허용한다. Studio와 새 예제는 항상 v1을 출력하며, 지원 목록에 없는 차트는 배포 전에 실패한다.

포맷 버전은 YAML 구조의 호환 경계다. 기존 reader가 안전하게 무시할 수 있는 선택 필드는 v1 안에서 추가할 수 있지만, 필드 의미 변경·삭제·필수화는 새 포맷 버전이 필요하다.

## 생성기 버전

Studio package metadata의 `generatorVersion`은 `crosswalk-vm` 구현 버전이며 현재 `1.0.0`이다.

- patch: 동일 입력의 오류 수정, 보안·결정성 개선
- minor: `crosswalk/v1` 안의 하위 호환 출력 기능 추가
- major: 생성 의미나 지원 범위의 비호환 변경

`chartFormat`은 loader 호환성, `generatorVersion`은 산출 로직 재현성을 나타내므로 서로 대신하지 않는다. package metadata의 `chartFormat`과 생성된 `deploy.yml`의 `format`은 항상 같아야 한다.

## 공개 검증 경계

배포 없이 실제 loader와 같은 검증을 실행한다.

```sh
deploy validate --config deploy.yml
```

Go 호출자는 `go-deployer/pkg/chart.ValidateFile`을 사용할 수 있다. 검증은 환경변수 치환, 포맷, 로컬 파일 존재 여부, 경로·포트·태그·bootstrap과 application 설정을 확인하며 SSH 연결이나 원격 변경은 수행하지 않는다.

로컬 파일 상대 경로는 현재 작업 디렉터리를 기준으로 해석되므로 package 검증은 `deploy.yml`이 포함된 디렉터리에서 실행한다.
