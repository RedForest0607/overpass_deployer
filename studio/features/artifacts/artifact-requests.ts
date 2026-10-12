export type ArtifactRequest = {
  projectId: string;
  kind: 'list' | 'upload';
  controller: AbortController;
};

/** One active request per mounted artifact workspace, including manual refresh. */
export class ArtifactRequests {
  private active: ArtifactRequest | null = null;

  begin(projectId: string, kind: ArtifactRequest['kind']) {
    if (this.active?.kind === 'upload') return null;
    this.cancel();
    const request = { projectId, kind, controller: new AbortController() };
    this.active = request;
    return request;
  }

  isCurrent(request: ArtifactRequest) {
    return this.active === request && !request.controller.signal.aborted;
  }

  finish(request: ArtifactRequest) {
    if (this.active === request) this.active = null;
  }

  cancel() {
    this.active?.controller.abort();
    this.active = null;
  }
}
