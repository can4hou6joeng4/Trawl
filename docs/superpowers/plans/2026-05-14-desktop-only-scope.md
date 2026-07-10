# Desktop-Only Scope Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the Web/Docker/API product surface and leave Legal Document Extractor as a Wails desktop application only.

**Architecture:** Delete unsupported server and Web spike entry points, simplify the Vue service layer to one Wails adapter, and update CI/release/docs to describe only macOS and Windows desktop distribution. Keep the Go extraction core and desktop UI behavior intact.

**Tech Stack:** Go 1.25, Wails v2, Vue 3, Vite, Vitest, GitHub Actions.

---

## File Map

- Delete `.air.toml`: server hot-reload config for removed Go/Echo API.
- Delete `.dockerignore`: Docker build config for removed Docker image.
- Delete `Dockerfile`: removed Web/Docker deployment.
- Delete `docker-compose.yml`: removed Web/Docker deployment.
- Delete `cmd/server/main.go`: removed Go/Echo REST API.
- Delete `spikes/nextjs-web/`: removed Next.js Web spike.
- Delete `docs/superpowers/specs/2026-05-13-nextjs-web-spike-design.md`: removed Next.js Web spike design.
- Delete `docs/superpowers/specs/2026-05-14-nextjs-web-spike-result.md`: removed Next.js Web spike result.
- Delete `docs/superpowers/plans/2026-05-14-nextjs-web-spike.md`: removed Next.js Web spike implementation plan.
- Delete `docs/contracts/extraction-response.md`: Web/API response contract no longer applies.
- Delete `docs/superpowers/specs/2026-05-14-web-edition-refactor-design.md`: superseded by desktop-only design.
- Delete `docs/superpowers/plans/2026-05-14-contract-and-infrastructure-baseline.md`: superseded by desktop-only plan.
- Modify `.github/workflows/ci.yml`: remove `./cmd/...` checks.
- Modify `.github/workflows/build.yml`: remove Docker job and release dependency.
- Modify `.github/ISSUE_TEMPLATE/bug_report.yml`: remove Docker/Web option.
- Modify `frontend/src/services/api.ts`: remove Web adapter and browser helper.
- Modify `frontend/src/services/api.test.ts`: replace with desktop-only service tests.
- Modify `frontend/src/App.vue`: use `string | null` selected file and remove Web export branches.
- Modify `frontend/src/components/MainDropZone.vue`: use local path strings and remove Web drop handling.
- Modify `frontend/src/components/MainDropZone.test.ts`: remove Web drop tests and cover Wails file drop.
- Modify `frontend/src/components/ConfigPanel.vue`: use local path strings and remove Web mode guard.
- Modify component tests that mock `api.isDesktop` or `api.isWeb` as needed.
- Modify `README.md`, `README.zh-CN.md`, `docs/user/CONFIG_GUIDE.md`, `docs/user/TROUBLESHOOTING.md`: remove current Web/Docker instructions.

---

### Task 1: Save Design And Plan

- [x] **Step 1: Create design and implementation plan**

Run:

```bash
test -f docs/superpowers/specs/2026-05-14-desktop-only-scope-design.md
test -f docs/superpowers/plans/2026-05-14-desktop-only-scope.md
```

Expected: both files exist.

---

### Task 2: Remove Server And Web Artifacts

- [x] **Step 1: Delete tracked Web/server files**

Run:

```bash
git rm -r .air.toml .dockerignore Dockerfile docker-compose.yml cmd/server spikes/nextjs-web docs/contracts/extraction-response.md docs/superpowers/specs/2026-05-13-nextjs-web-spike-design.md docs/superpowers/specs/2026-05-14-nextjs-web-spike-result.md docs/superpowers/specs/2026-05-14-web-edition-refactor-design.md docs/superpowers/plans/2026-05-14-nextjs-web-spike.md docs/superpowers/plans/2026-05-14-contract-and-infrastructure-baseline.md
```

Expected: files are staged as deleted.

- [x] **Step 2: Remove untracked generated spike artifacts if present**

Run:

```bash
rm -rf spikes/nextjs-web/.next spikes/nextjs-web/node_modules spikes/nextjs-web/tsconfig.tsbuildinfo
```

Expected: command exits `0`.

---

### Task 3: Update CI And Release Workflows

- [x] **Step 1: Update `.github/workflows/ci.yml`**

Change the Go lint args from:

```yaml
args: ./internal/... ./cmd/...
```

to:

```yaml
args: ./internal/... .
```

Change tests from:

```yaml
go test -race -count=1 -coverprofile=coverage.out -covermode=atomic ./internal/... ./cmd/...
```

to:

```yaml
go test -race -count=1 -coverprofile=coverage.out -covermode=atomic ./internal/... .
```

Change build check from:

```yaml
go build ./internal/... ./cmd/...
```

to:

```yaml
go build ./internal/... .
```

- [x] **Step 2: Update `.github/workflows/build.yml`**

Delete the entire `build-docker` job.

Change:

```yaml
  release:
    needs: [build-macos, build-windows, build-docker]
```

to:

```yaml
  release:
    needs: [build-macos, build-windows]
```

- [x] **Step 3: Update issue template**

Remove `Docker / Web` from `.github/ISSUE_TEMPLATE/bug_report.yml`.

---

### Task 4: Simplify Desktop Service Layer

- [x] **Step 1: Replace `frontend/src/services/api.ts`**

Replace it with a desktop-only Wails adapter:

```ts
export interface Record {
  [key: string]: string;
}

export interface ExtractResult {
  success: boolean;
  recordCount: number;
  outputPath?: string;
  errorMessage?: string;
  records?: Record[];
  fieldLabels?: { [key: string]: string };
}

export interface FieldOption {
  key: string;
  label: string;
}

export interface IApiService {
  selectFile(): Promise<string>;
  previewData(filePath: string, fields: string[]): Promise<ExtractResult>;
  extractToPath(filePath: string, outputPath: string, fields: string[]): Promise<ExtractResult>;
  exportData(records: Record[], outputPath: string): Promise<ExtractResult>;
  selectOutputPath(defaultName: string): Promise<string>;
  scanFields(filePath: string): Promise<FieldOption[]>;
  openFile(path: string): Promise<void>;
  getTrialStatus(): Promise<any>;
  getMachineID(): Promise<string>;
  activate(licenseKey: string): Promise<boolean>;
  cancelExtraction(): Promise<void>;
}

class DesktopAdapter implements IApiService {
  async selectFile(): Promise<string> {
    const { SelectFile } = await import('../../wailsjs/go/app/App');
    return SelectFile();
  }

  async previewData(filePath: string, fields: string[]): Promise<ExtractResult> {
    const { PreviewData } = await import('../../wailsjs/go/app/App');
    return PreviewData(filePath, fields);
  }

  async extractToPath(filePath: string, outputPath: string, fields: string[]): Promise<ExtractResult> {
    const { ExtractToPath } = await import('../../wailsjs/go/app/App');
    return ExtractToPath(filePath, outputPath, fields);
  }

  async exportData(records: Record[], outputPath: string): Promise<ExtractResult> {
    const { ExportData } = await import('../../wailsjs/go/app/App');
    return ExportData(records, outputPath);
  }

  async selectOutputPath(defaultName: string): Promise<string> {
    const { SelectOutputPath } = await import('../../wailsjs/go/app/App');
    return SelectOutputPath(defaultName);
  }

  async scanFields(filePath: string): Promise<FieldOption[]> {
    const { ScanFields } = await import('../../wailsjs/go/app/App');
    return ScanFields(filePath);
  }

  async openFile(path: string): Promise<void> {
    const { OpenFile } = await import('../../wailsjs/go/app/App');
    return OpenFile(path);
  }

  async getTrialStatus(): Promise<any> {
    const { GetTrialStatus } = await import('../../wailsjs/go/app/App');
    return GetTrialStatus();
  }

  async getMachineID(): Promise<string> {
    const { GetMachineID } = await import('../../wailsjs/go/app/App');
    return GetMachineID();
  }

  async activate(licenseKey: string): Promise<boolean> {
    const { Activate } = await import('../../wailsjs/go/app/App');
    return Activate(licenseKey);
  }

  async cancelExtraction(): Promise<void> {
    const { CancelExtraction } = await import('../../wailsjs/go/app/App');
    return CancelExtraction();
  }
}

let apiServiceInstance: IApiService | null = null;

export function getApiService(): IApiService {
  if (!apiServiceInstance) {
    apiServiceInstance = new DesktopAdapter();
  }
  return apiServiceInstance;
}

export const api = {
  get isDesktop() {
    return true;
  },
  get service() {
    return getApiService();
  },
};
```

- [x] **Step 2: Replace `frontend/src/services/api.test.ts`**

Use tests that assert `api.isDesktop` and Wails methods are loaded through dynamic imports:

```ts
import { beforeEach, describe, expect, it, vi } from 'vitest';

const wailsMethods = vi.hoisted(() => ({
  SelectFile: vi.fn(),
  PreviewData: vi.fn(),
  ExtractToPath: vi.fn(),
  ExportData: vi.fn(),
  SelectOutputPath: vi.fn(),
  ScanFields: vi.fn(),
  OpenFile: vi.fn(),
  GetTrialStatus: vi.fn(),
  GetMachineID: vi.fn(),
  Activate: vi.fn(),
  CancelExtraction: vi.fn(),
}));

vi.mock('../../wailsjs/go/app/App', () => wailsMethods);

describe('desktop api service', () => {
  beforeEach(() => {
    vi.resetModules();
    Object.values(wailsMethods).forEach(method => method.mockReset());
  });

  it('always reports desktop mode', async () => {
    const { api } = await import('./api');
    expect(api.isDesktop).toBe(true);
    expect('isWeb' in api).toBe(false);
  });

  it('delegates file selection to Wails', async () => {
    wailsMethods.SelectFile.mockResolvedValue('/tmp/case.pdf');
    const { getApiService } = await import('./api');
    await expect(getApiService().selectFile()).resolves.toBe('/tmp/case.pdf');
    expect(wailsMethods.SelectFile).toHaveBeenCalledTimes(1);
  });

  it('delegates cancellation to Wails', async () => {
    wailsMethods.CancelExtraction.mockResolvedValue(undefined);
    const { getApiService } = await import('./api');
    await getApiService().cancelExtraction();
    expect(wailsMethods.CancelExtraction).toHaveBeenCalledTimes(1);
  });
});
```

---

### Task 5: Remove Web Branches From Vue Components

- [x] **Step 1: Update `frontend/src/App.vue`**

Remove `downloadBlob` import. Change:

```ts
const selectedFile = ref<string | File | null>(null);
```

to:

```ts
const selectedFile = ref<string | null>(null);
```

Simplify `fileName` so it only handles strings. Change `handleFileUpdate(file: string | File)` to `handleFileUpdate(file: string)`.

In `handleExtract`, remove the `api.isDesktop` conditional and always select or reuse a desktop output path before calling `extractToPath`. Remove the `api.isWeb` export/download block.

Remove `if (api.isWeb) return;` guards from `handleSelectOutputPath` and `handleOpenFile`.

Change the trial banner condition from:

```vue
<div v-if="trialStatus && api.isDesktop && !trialStatus.isActivated"
```

to:

```vue
<div v-if="trialStatus && !trialStatus.isActivated"
```

- [x] **Step 2: Update `frontend/src/components/MainDropZone.vue`**

Change props and emits from `string | File` to `string`. Remove `File` display-size handling. Remove `handleWebDrop`. Keep the Wails `OnFileDrop` behavior and always register it in `onMounted`.

- [x] **Step 3: Update `frontend/src/components/ConfigPanel.vue`**

Change `selectedFile: string | File` to `selectedFile: string`. Remove the `api` import if it is only used for `api.isWeb`. Remove the `if (api.isWeb) return;` branch from `handleSelectOutput`.

- [x] **Step 4: Update component tests**

Remove Web-specific test cases from `MainDropZone.test.ts`; add a Wails drop test by mocking `../../wailsjs/runtime/runtime`.

Update mocks in `ResultCard.test.ts` and `PreviewTable.test.ts` to remove `isDesktop: false`.

---

### Task 6: Update Current Documentation

- [x] **Step 1: Update README files**

Remove Web/Docker badge, feature, quick start, prerequisites, dev-mode, and project-structure entries from `README.md` and `README.zh-CN.md`.

Set platform badge to macOS and Windows only.

Set prerequisites to Go 1.25+, Node.js 22+, and Wails CLI.

- [x] **Step 2: Update user docs**

In `docs/user/CONFIG_GUIDE.md`, change the environment variable section title from Docker/development to desktop development and use `LEGAL_EXTRACTOR_BAIDU_TOKEN`.

In `docs/user/TROUBLESHOOTING.md`, remove Web/Docker token troubleshooting and Docker restart commands. Update Go version requirement to Go 1.25+.

---

### Task 7: Verify Removal

- [x] **Step 1: Search for removed active references**

Run:

```bash
rg -n "WebAdapter|isWebMode|VITE_API_URL|/api/extract|/api/export|spikes/nextjs-web|docker-compose|build-docker|cmd/server" README.md README.zh-CN.md docs/user .github frontend/src main.go internal
```

Expected: no matches.

- [x] **Step 2: Run Go tests**

Run:

```bash
go test ./... -count=1
```

Expected: all remaining Go packages pass.

- [x] **Step 3: Run Go build**

Run:

```bash
go build . ./internal/...
```

Expected: build passes.

- [x] **Step 4: Run frontend tests**

Run:

```bash
npm test
```

Working directory: `frontend`

Expected: all frontend tests pass.

- [x] **Step 5: Run frontend build**

Run:

```bash
npm run build
```

Working directory: `frontend`

Expected: production build passes.

---

### Task 8: Commit Desktop-Only Cleanup

- [x] **Step 1: Review final status**

Run:

```bash
git diff --stat
git status --short --branch
```

Expected: only desktop-only removal and documentation files changed.

- [x] **Step 2: Commit**

Run:

```bash
git add -A
git commit -m "refactor: 收敛为桌面端应用"
```

Expected: commit succeeds.
