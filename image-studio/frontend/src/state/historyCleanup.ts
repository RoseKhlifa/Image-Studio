import type { StudioState } from "./studioStore.types";
import type { SavePromptRequest } from "../lib/savePromptState";

type HistoryCleanupState = Pick<
  StudioState,
  | "history"
  | "historyHasMore"
  | "historyLoading"
  | "historyCursorBeforeDayStart"
  | "currentImage"
  | "batchResults"
  | "resultGridOpen"
  | "compareB"
  | "resultDetail"
  | "savePromptRequest"
  | "savePromptQueue"
  | "workspaces"
>;

type HistoryLoadState = Pick<StudioState, "historyLoading" | "loadMoreHistory">;

export async function waitForActiveHistoryLoad(
  getState: () => HistoryLoadState,
): Promise<void> {
  const state = getState();
  if (!state.historyLoading) return;
  await state.loadMoreHistory();
}

function filterSavePromptRequest(
  request: SavePromptRequest | null,
  cleared: ReadonlySet<string>,
): SavePromptRequest | null {
  if (!request) return null;
  if (request.kind === "single") {
    return cleared.has(request.item.id) ? null : request;
  }
  const items = request.items.filter((item) => !cleared.has(item.id));
  return items.length > 0 ? { ...request, items } : null;
}

export function buildHistoryCleanupPatch(
  state: HistoryCleanupState,
  clearedIDs: Iterable<string>,
): HistoryCleanupState {
  const cleared = new Set(clearedIDs);
  const history = state.history.filter((item) => !cleared.has(item.id));
  const batchResults = state.batchResults.filter((item) => !cleared.has(item.id));
  const filteredQueue = state.savePromptQueue
    .map((request) => filterSavePromptRequest(request, cleared))
    .filter((request): request is SavePromptRequest => request !== null);
  let savePromptRequest = filterSavePromptRequest(state.savePromptRequest, cleared);
  if (!savePromptRequest && filteredQueue.length > 0) {
    savePromptRequest = filteredQueue.shift() ?? null;
  }

  return {
    history,
    historyHasMore: false,
    historyLoading: false,
    historyCursorBeforeDayStart: null,
    currentImage: state.currentImage && cleared.has(state.currentImage.id) ? null : state.currentImage,
    batchResults,
    resultGridOpen: batchResults.length > 1 && state.resultGridOpen,
    compareB: state.compareB && cleared.has(state.compareB.id) ? null : state.compareB,
    resultDetail: state.resultDetail && cleared.has(state.resultDetail.id) ? null : state.resultDetail,
    savePromptRequest,
    savePromptQueue: filteredQueue,
    workspaces: state.workspaces.map((workspace) => {
      const batchResultIds = workspace.batchResultIds.filter((id) => !cleared.has(id));
      return {
        ...workspace,
        currentImageId: workspace.currentImageId && cleared.has(workspace.currentImageId)
          ? null
          : workspace.currentImageId,
        batchResultIds,
        resultGridOpen: batchResultIds.length > 1 && workspace.resultGridOpen,
      };
    }),
  };
}
