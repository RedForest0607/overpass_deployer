'use client';

import { useEffect, useRef, useState } from 'react';
import type { Environment } from '../../lib/api/contracts';
import type { Project } from '../../lib/api/projects';
import type { usePersistence } from './usePersistence';
import {
  isSameTopologyContext,
  requiresUnsavedConfirmation,
} from './unsaved-navigation';

type Destination = { project: Project; environment: Environment };
type Persistence = Pick<
  ReturnType<typeof usePersistence>,
  'active' | 'environment' | 'dirty' | 'busy' | 'open' | 'save'
>;

export function useTopologyNavigation(
  persistence: Persistence,
  onOpened: () => void,
) {
  const [pendingNavigation, setPendingNavigation] =
    useState<Destination | null>(null);
  const [navigationAttempted, setNavigationAttempted] = useState(false);
  const lock = useRef(false);
  const alive = useRef(true);
  useEffect(() => {
    alive.current = true;
    return () => {
      alive.current = false;
    };
  }, []);

  async function navigate(
    target: Destination,
    saveFirst: boolean,
    discardChanges: boolean,
  ) {
    if (lock.current || persistence.busy) return;
    lock.current = true;
    setNavigationAttempted(true);
    try {
      if (saveFirst && !(await persistence.save())) return;
      if (!alive.current) return;
      const opened = await persistence.open(
        target.project,
        target.environment,
        { discardChanges },
      );
      if (!opened || !alive.current) return;
      setPendingNavigation(null);
      setNavigationAttempted(false);
      onOpened();
    } finally {
      lock.current = false;
    }
  }

  function requestTopology(project: Project, environment: Environment) {
    if (lock.current || persistence.busy) return;
    const current = {
      projectId: persistence.active?.id,
      environment: persistence.environment,
    };
    const target = { projectId: project.id, environment };
    if (isSameTopologyContext(current, target)) {
      onOpened();
      return;
    }
    if (requiresUnsavedConfirmation(persistence.dirty, current, target)) {
      setNavigationAttempted(false);
      setPendingNavigation({ project, environment });
      return;
    }
    void navigate({ project, environment }, false, false);
  }

  function cancelNavigation() {
    if (lock.current || persistence.busy) return;
    setPendingNavigation(null);
    setNavigationAttempted(false);
  }

  return {
    pendingNavigation,
    navigationAttempted,
    requestTopology,
    cancelNavigation,
    saveAndNavigate: () =>
      pendingNavigation
        ? navigate(pendingNavigation, true, true)
        : Promise.resolve(),
    discardAndNavigate: () =>
      pendingNavigation
        ? navigate(pendingNavigation, false, true)
        : Promise.resolve(),
  };
}
