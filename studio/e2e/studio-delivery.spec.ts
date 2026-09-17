import {
  expect,
  test,
  type APIRequestContext,
  type Page,
  type TestInfo,
} from '@playwright/test';
import { readFile, writeFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import JSZip from 'jszip';

const e2eDir = path.dirname(fileURLToPath(import.meta.url));
const fixtureDir = path.join(e2eDir, 'fixtures');
const topologyPath = path.join(fixtureDir, 'minimal-topology.json');
const generatorFixtureDir = path.resolve(
  e2eDir,
  '../backend/internal/generator/crosswalkvm/testdata/loader-compat',
);

function uniqueName(prefix: string) {
  return `${prefix}-${Date.now()}-${Math.random().toString(16).slice(2, 8)}`;
}

async function createJarFixture(testInfo: TestInfo) {
  const archive = new JSZip();
  archive.file(
    'META-INF/MANIFEST.MF',
    'Manifest-Version: 1.0\nMain-Class: example.Application\n',
  );
  archive.file('example/Application.class', 'browser-e2e-placeholder');
  const jarPath = testInfo.outputPath('commerce.jar');
  await writeFile(jarPath, await archive.generateAsync({ type: 'nodebuffer' }));
  return jarPath;
}

async function openWorkspace(page: Page, projectName: string) {
  await page.goto('/');
  await expect(page.getByRole('status')).toHaveText(
    '프로젝트를 생성해 시작하세요.',
  );
  await page.getByRole('button', { name: '프로젝트', exact: true }).click();
  await page.getByRole('button', { name: '프로젝트 생성' }).click();
  const sheet = page.getByRole('dialog');
  await sheet.getByLabel('프로젝트 이름').fill(projectName);
  await sheet.getByLabel('설명').fill('Playwright browser E2E');
  await sheet.getByRole('button', { name: '저장' }).click();
  await expect(page.getByRole('heading', { name: projectName })).toBeVisible();
}

async function uploadArtifact(
  page: Page,
  kindLabel: string,
  filePath: string,
  expectedName: string,
) {
  await page.getByRole('combobox', { name: '파일 종류' }).click();
  await page.getByRole('option', { name: kindLabel }).click();
  await page.locator('#artifact-file').setInputFiles(filePath);
  await page.getByRole('button', { name: '등록', exact: true }).click();
  await expect(page.getByText('파일을 안전하게 등록했습니다.')).toBeVisible();
  await expect(page.getByRole('cell', { name: expectedName })).toBeVisible();
}

async function createFailedGeneration(
  request: APIRequestContext,
  projectName: string,
) {
  const created = await request.post('/api/v1/projects', {
    data: { name: projectName, description: 'failed generation fixture' },
  });
  expect(created.status()).toBe(201);
  const location = created.headers().location;
  expect(location).toBeTruthy();
  const projectId = location.split('/').at(-1)!;
  const document = JSON.parse(await readFile(topologyPath, 'utf8'));
  const saved = await request.put(
    `/api/v1/projects/${projectId}/topologies/dev`,
    {
      data: {
        expectedRevision: 0,
        blueprint: document.blueprint,
        layout: document.layout,
      },
    },
  );
  expect(saved.status()).toBe(200);
  const generated = await request.post(
    `/api/v1/projects/${projectId}/generations`,
    {
      headers: { 'Idempotency-Key': uniqueName('failed-generation') },
      data: {
        environment: 'dev',
        revision: 1,
        generator: 'crosswalk-vm',
        output: 'package',
      },
    },
  );
  expect(generated.status()).toBe(202);
  const generation = await generated.json();
  await expect
    .poll(async () => {
      const response = await request.get(
        `/api/v1/projects/${projectId}/generations/${generation.id}`,
      );
      return (await response.json()).status;
    })
    .toBe('failed');
  return { projectId, generationId: generation.id };
}

test('uploads artifacts, maps a topology, saves it, and verifies the downloaded package', async ({
  page,
}, testInfo) => {
  const projectName = uniqueName('browser-delivery');
  const jarPath = await createJarFixture(testInfo);
  await openWorkspace(page, projectName);

  await page.getByRole('button', { name: '파일', exact: true }).click();
  await uploadArtifact(page, 'Java JAR', jarPath, 'commerce.jar');
  await uploadArtifact(
    page,
    '설정 템플릿',
    path.join(generatorFixtureDir, 'application.yml'),
    'application.yml',
  );

  await page.getByRole('button', { name: '토폴로지', exact: true }).click();
  await page.getByRole('button', { name: 'JSON 가져오기' }).click();
  const importDialog = page.getByRole('dialog');
  await importDialog.locator('input[type=file]').setInputFiles(topologyPath);
  await importDialog.getByRole('button', { name: '에디터에 적용' }).click();
  await expect(
    page.getByText(/minimal-topology\.json을 에디터에 적용/),
  ).toBeVisible();

  await page.getByRole('button', { name: '파일', exact: true }).click();
  await expect(page.getByText('0/2 연결')).toBeVisible();
  await page.getByRole('button', { name: '추천 선택' }).click();
  await page.getByRole('button', { name: '매핑 적용' }).click();
  await expect(page.getByText(/2개 참조를 적용했습니다/)).toBeVisible();

  await page.getByRole('button', { name: '토폴로지', exact: true }).click();
  await page.getByRole('button', { name: '설계 저장' }).click();
  await expect(page.getByText('설계를 저장했습니다. revision 1')).toBeVisible();

  await page.getByRole('button', { name: '배포 패키지', exact: true }).click();
  await page.getByRole('button', { name: '검증 후 생성' }).click();
  await expect(
    page.getByText(/revision 1 기반 패키지를 생성했습니다/),
  ).toBeVisible({
    timeout: 30_000,
  });
  await expect(page.getByText('생성 이력')).toBeVisible();
  await expect(page.getByText('완료', { exact: true }).first()).toBeVisible();

  const downloadPromise = page.waitForEvent('download');
  await page.getByRole('button', { name: '실제 ZIP 다운로드' }).click();
  const download = await downloadPromise;
  const downloadedPath = await download.path();
  expect(downloadedPath).toBeTruthy();
  const archive = await JSZip.loadAsync(await readFile(downloadedPath!));
  for (const expectedPath of [
    'deploy.yml',
    'assets/commerce/commerce.jar',
    'config/commerce/application.yml',
  ]) {
    expect(archive.files[expectedPath], `missing ${expectedPath}`).toBeTruthy();
  }
});

test('retries a failed immutable snapshot from generation history', async ({
  page,
  request,
}) => {
  const projectName = uniqueName('browser-retry');
  const fixture = await createFailedGeneration(request, projectName);

  await page.goto(
    `/?project=${encodeURIComponent(fixture.projectId)}&environment=dev`,
  );
  await expect(page.getByRole('heading', { name: projectName })).toBeVisible();
  await page.getByRole('button', { name: '배포 패키지', exact: true }).click();
  await expect(page.getByText('생성 이력')).toBeVisible();
  await expect(page.getByText('실패', { exact: true }).first()).toBeVisible();

  const retryResponse = page.waitForResponse(
    (response) =>
      response.request().method() === 'POST' &&
      response.url().endsWith(`/generations/${fixture.generationId}/retry`),
  );
  await page.getByRole('button', { name: 'snapshot 재시도' }).first().click();
  const retried = await (await retryResponse).json();
  expect(retried.retryOfGenerationId).toBe(fixture.generationId);
  await expect(page.getByText('재시도 작업').first()).toBeVisible({
    timeout: 30_000,
  });
  await expect
    .poll(async () => {
      const response = await request.get(
        `/api/v1/projects/${fixture.projectId}/generations?environment=dev&limit=10`,
      );
      const body = await response.json();
      return body.items.filter(
        (item: { retryOfGenerationId: string | null }) =>
          item.retryOfGenerationId === fixture.generationId,
      ).length;
    })
    .toBe(1);
});
