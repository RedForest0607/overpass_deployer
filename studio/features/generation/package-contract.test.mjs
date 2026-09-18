import assert from 'node:assert/strict';
import test from 'node:test';
import {
  evaluatePackageCapability,
  packageContractError,
  REQUIRED_CHART_FORMAT,
} from './package-contract.ts';

const capabilities = (generator) => ({
  blueprintVersions: ['studio.deployer/v1'],
  generators: generator ? [generator] : [],
  limits: {},
});

test('package generation requires the crosswalk-vm package output and v1 chart format', () => {
  assert.deepEqual(evaluatePackageCapability(null), {
    supported: false,
    reason: null,
  });
  assert.match(
    evaluatePackageCapability(capabilities(null)).reason,
    /crosswalk-vm/,
  );
  assert.match(
    evaluatePackageCapability(
      capabilities({
        id: 'crosswalk-vm',
        outputs: ['draft'],
        chartFormats: [REQUIRED_CHART_FORMAT],
      }),
    ).reason,
    /package/,
  );
  assert.match(
    evaluatePackageCapability(
      capabilities({ id: 'crosswalk-vm', outputs: ['package'] }),
    ).reason,
    /crosswalk\/v1/,
  );
  assert.deepEqual(
    evaluatePackageCapability(
      capabilities({
        id: 'crosswalk-vm',
        outputs: ['package'],
        chartFormats: [REQUIRED_CHART_FORMAT],
      }),
    ),
    { supported: true, reason: null },
  );
});

test('Docker package generation uses an independent capability', () => {
  assert.match(
    evaluatePackageCapability(capabilities(null), 'crosswalk-docker').reason,
    /crosswalk-docker/,
  );
  assert.deepEqual(
    evaluatePackageCapability(
      capabilities({
        id: 'crosswalk-docker',
        outputs: ['package'],
        chartFormats: [REQUIRED_CHART_FORMAT],
      }),
      'crosswalk-docker',
    ),
    { supported: true, reason: null },
  );
});

test('Compose package generation uses an independent capability', () => {
  assert.deepEqual(
    evaluatePackageCapability(
      capabilities({
        id: 'crosswalk-compose',
        outputs: ['package'],
        chartFormats: [REQUIRED_CHART_FORMAT],
      }),
      'crosswalk-compose',
    ),
    { supported: true, reason: null },
  );
  assert.equal(
    packageContractError(
      {
        chartFormat: REQUIRED_CHART_FORMAT,
        generator: 'crosswalk-compose',
        generatorVersion: '1.0.0',
      },
      'crosswalk-compose',
    ),
    null,
  );
});

test('package responses with missing or incompatible chart formats are rejected', () => {
  assert.equal(
    packageContractError({
      chartFormat: REQUIRED_CHART_FORMAT,
      generator: 'crosswalk-vm',
      generatorVersion: '1.0.0',
    }),
    null,
  );
  assert.match(packageContractError({}), /표시 없음/);
  assert.match(
    packageContractError({ chartFormat: 'crosswalk/v2' }),
    /crosswalk\/v2/,
  );
  assert.match(
    packageContractError({
      chartFormat: REQUIRED_CHART_FORMAT,
      generator: 'other',
      generatorVersion: '1.0.0',
    }),
    /지원 계약/,
  );
  assert.equal(
    packageContractError(
      {
        chartFormat: REQUIRED_CHART_FORMAT,
        generator: 'crosswalk-docker',
        generatorVersion: '1.0.0',
      },
      'crosswalk-docker',
    ),
    null,
  );
  assert.match(
    packageContractError(
      {
        chartFormat: REQUIRED_CHART_FORMAT,
        generator: 'crosswalk-vm',
        generatorVersion: '1.0.0',
      },
      'crosswalk-docker',
    ),
    /crosswalk-docker/,
  );
  assert.match(
    packageContractError({
      chartFormat: REQUIRED_CHART_FORMAT,
      generator: 'crosswalk-vm',
    }),
    /버전/,
  );
});
