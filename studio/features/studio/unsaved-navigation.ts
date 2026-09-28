import type { Environment } from '../../lib/api/contracts';

export type TopologyContext = {
  projectId?: string;
  environment: Environment;
};

export function isSameTopologyContext(
  current: TopologyContext,
  target: TopologyContext,
) {
  return (
    current.projectId === target.projectId &&
    current.environment === target.environment
  );
}

export function requiresUnsavedConfirmation(
  dirty: boolean,
  current: TopologyContext,
  target: TopologyContext,
) {
  return dirty && !isSameTopologyContext(current, target);
}
