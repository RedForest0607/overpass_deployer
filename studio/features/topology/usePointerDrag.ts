'use client';

import {
  useRef,
  type MouseEvent as ReactMouseEvent,
  type PointerEvent as ReactPointerEvent,
} from 'react';

type PointerDragOptions = {
  onDrag: (point: { x: number; y: number }) => void;
  onClick?: () => void;
  threshold?: number;
};

type DragState = {
  pointerId: number;
  startX: number;
  startY: number;
  dragged: boolean;
};

export function usePointerDrag({
  onDrag,
  onClick,
  threshold = 5,
}: PointerDragOptions) {
  const state = useRef<DragState | null>(null);
  const suppressNextClick = useRef(false);
  const callbacks = useRef({ onDrag, onClick });
  callbacks.current = { onDrag, onClick };

  function finish(
    event: ReactPointerEvent<HTMLButtonElement>,
    cancelled: boolean,
  ) {
    const current = state.current;
    if (!current || current.pointerId !== event.pointerId) return;
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
    state.current = null;
    event.preventDefault();
    event.stopPropagation();
    suppressNextClick.current = true;
    window.setTimeout(() => {
      suppressNextClick.current = false;
    }, 0);
    if (!cancelled && !current.dragged) callbacks.current.onClick?.();
  }

  return {
    onPointerDown(event: ReactPointerEvent<HTMLButtonElement>) {
      event.preventDefault();
      event.stopPropagation();
      state.current = {
        pointerId: event.pointerId,
        startX: event.clientX,
        startY: event.clientY,
        dragged: false,
      };
      event.currentTarget.setPointerCapture(event.pointerId);
    },
    onPointerMove(event: ReactPointerEvent<HTMLButtonElement>) {
      const current = state.current;
      if (!current || current.pointerId !== event.pointerId) return;
      if (
        !current.dragged &&
        !exceedsDragThreshold(
          { x: current.startX, y: current.startY },
          { x: event.clientX, y: event.clientY },
          threshold,
        )
      )
        return;
      current.dragged = true;
      event.preventDefault();
      event.stopPropagation();
      callbacks.current.onDrag({ x: event.clientX, y: event.clientY });
    },
    onPointerUp(event: ReactPointerEvent<HTMLButtonElement>) {
      finish(event, false);
    },
    onPointerCancel(event: ReactPointerEvent<HTMLButtonElement>) {
      finish(event, true);
    },
    onClick(event: ReactMouseEvent<HTMLButtonElement>) {
      event.preventDefault();
      event.stopPropagation();
      if (!suppressNextClick.current) callbacks.current.onClick?.();
    },
  };
}

export function exceedsDragThreshold(
  start: { x: number; y: number },
  current: { x: number; y: number },
  threshold: number,
) {
  return Math.hypot(current.x - start.x, current.y - start.y) >= threshold;
}
