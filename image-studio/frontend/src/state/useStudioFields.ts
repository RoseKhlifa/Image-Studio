import { useShallow } from "zustand/react/shallow";
import { useStudioStore } from "./studioStore";
import type { StudioState } from "./studioStore.types";

// Component subscriptions must include every field read during rendering.
// Actions and getState() calls made inside event handlers can stay stable.
export function useStudioFields<Key extends keyof StudioState>(keys: readonly Key[]): Pick<StudioState, Key> {
  return useStudioStore(useShallow((state) => {
    const selected = {} as Pick<StudioState, Key>;
    for (const key of keys) selected[key] = state[key];
    return selected;
  }));
}
