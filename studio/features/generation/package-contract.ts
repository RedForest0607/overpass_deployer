import type { Capabilities, Package } from '../../lib/api/generated.ts';
import type { GeneratorID } from '../../lib/api/contracts.ts';

export const REQUIRED_CHART_FORMAT = 'crosswalk/v1' as const;
export const DEFAULT_GENERATOR: GeneratorID = 'crosswalk-vm';

export type PackageCapability = {
  supported: boolean;
  reason: string | null;
};

export function evaluatePackageCapability(
  capabilities: Capabilities | null,
  generatorID: GeneratorID = DEFAULT_GENERATOR,
): PackageCapability {
  if (!capabilities) return { supported: false, reason: null };

  const generator = capabilities.generators.find(
    (candidate) => candidate.id === generatorID,
  );
  if (!generator || !generator.outputs?.includes('package')) {
    return {
      supported: false,
      reason: `서버에서 ${generatorID} package 생성을 지원하지 않습니다.`,
    };
  }
  if (
    !Array.isArray(generator.chartFormats) ||
    !generator.chartFormats.includes(REQUIRED_CHART_FORMAT)
  ) {
    return {
      supported: false,
      reason:
        '서버가 crosswalk/v1 차트 생성을 지원하지 않습니다. 서버를 업데이트하세요.',
    };
  }
  return { supported: true, reason: null };
}

export function packageContractError(
  value:
    | Pick<Package, 'chartFormat' | 'generator' | 'generatorVersion'>
    | {
        chartFormat?: unknown;
        generator?: unknown;
        generatorVersion?: unknown;
      },
  expectedGenerator?: GeneratorID,
): string | null {
  if (value.chartFormat !== REQUIRED_CHART_FORMAT) {
    const received =
      typeof value.chartFormat === 'string' && value.chartFormat.trim()
        ? value.chartFormat
        : '표시 없음';
    return `지원하지 않는 차트 포맷을 반환했습니다: ${received}. 필요한 포맷은 ${REQUIRED_CHART_FORMAT}입니다.`;
  }
  if (
    value.generator !== 'crosswalk-vm' &&
    value.generator !== 'crosswalk-docker' &&
    value.generator !== 'crosswalk-compose'
  ) {
    return '패키지 생성기 정보가 지원 계약과 일치하지 않습니다.';
  }
  if (expectedGenerator && value.generator !== expectedGenerator) {
    return `패키지 생성기 정보가 ${expectedGenerator} 계약과 일치하지 않습니다.`;
  }
  if (
    typeof value.generatorVersion !== 'string' ||
    !value.generatorVersion.trim()
  ) {
    return '패키지 생성기 버전 정보가 없습니다.';
  }
  return null;
}
