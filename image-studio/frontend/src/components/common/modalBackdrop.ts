export type BackdropPointerGesture = {
  pointerId: number;
  startedOnBackdrop: boolean;
};

export function beginBackdropPointerGesture(
  pointerId: number,
  startedOnBackdrop: boolean,
): BackdropPointerGesture {
  return { pointerId, startedOnBackdrop };
}

export function shouldDismissFromBackdropPointer(
  gesture: BackdropPointerGesture | null,
  pointerId: number,
  endedOnBackdrop: boolean,
): boolean {
  return gesture?.pointerId === pointerId
    && gesture.startedOnBackdrop
    && endedOnBackdrop;
}
