import * as React from 'react';

const MOBILE_BREAKPOINT = 768;

const mediaQuery = `(max-width: ${MOBILE_BREAKPOINT - 1}px)`;

function subscribe(onChange: () => void) {
  const media = window.matchMedia(mediaQuery);
  media.addEventListener('change', onChange);
  return () => media.removeEventListener('change', onChange);
}

export function useIsMobile() {
  return React.useSyncExternalStore(
    subscribe,
    () => window.matchMedia(mediaQuery).matches,
    () => false,
  );
}
