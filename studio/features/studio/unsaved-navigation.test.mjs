import assert from 'node:assert/strict';
import test from 'node:test';
import {
  isSameTopologyContext,
  requiresUnsavedConfirmation,
} from './unsaved-navigation.ts';

const projectADev = { projectId: 'project-a', environment: 'dev' };

void test('requires confirmation before opening another project with unsaved changes', () => {
  assert.equal(
    requiresUnsavedConfirmation(true, projectADev, {
      projectId: 'project-b',
      environment: 'dev',
    }),
    true,
  );
});

void test('requires confirmation before changing environment with unsaved changes', () => {
  assert.equal(
    requiresUnsavedConfirmation(true, projectADev, {
      projectId: 'project-a',
      environment: 'qa',
    }),
    true,
  );
});

void test('keeps the current edit when reopening the same topology context', () => {
  assert.equal(isSameTopologyContext(projectADev, projectADev), true);
  assert.equal(
    requiresUnsavedConfirmation(true, projectADev, projectADev),
    false,
  );
});

void test('allows context changes when there are no unsaved changes', () => {
  assert.equal(
    requiresUnsavedConfirmation(false, projectADev, {
      projectId: 'project-b',
      environment: 'prod',
    }),
    false,
  );
});
