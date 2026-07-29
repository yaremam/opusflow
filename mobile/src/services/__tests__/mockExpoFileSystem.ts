// A small in-memory fake of expo-file-system's class-based API (File,
// Directory, Paths) for tests — real device I/O isn't available under
// vitest, and this codebase's convention (see connection.test.ts mocking
// expo-secure-store) is a per-module vi.mock rather than relying on any
// framework auto-mock. Modeled on the *behavior* of expo-file-system's own
// real classes (create/write/text/delete/exists), not a stub.

interface Entry {
  kind: 'file' | 'dir';
  bytes: Uint8Array;
}

export const mockFs = {
  store: new Map<string, Entry>(),
  availableDiskSpace: 500 * 1024 * 1024 * 1024, // 500 GB by default; tests override as needed
  downloadedByteLength: 1024, // how many bytes a "downloaded" file mock-contains
  lastDownloadHeaders: undefined as Record<string, string> | undefined,
};

export function resetMockFileSystem() {
  mockFs.store.clear();
  mockFs.availableDiskSpace = 500 * 1024 * 1024 * 1024;
  mockFs.downloadedByteLength = 1024;
  mockFs.lastDownloadHeaders = undefined;
}

function normalize(uri: string): string {
  return uri.replace(/\/+$/, '');
}

class MockDirectory {
  uri: string;
  constructor(...parts: (string | MockDirectory | MockFile)[]) {
    this.uri = normalize(parts.map((p) => (typeof p === 'string' ? p : p.uri)).join('/'));
  }
  get exists(): boolean {
    return mockFs.store.get(this.uri)?.kind === 'dir';
  }
  create(_options?: { intermediates?: boolean; idempotent?: boolean }) {
    if (!mockFs.store.has(this.uri)) {
      mockFs.store.set(this.uri, { kind: 'dir', bytes: new Uint8Array() });
    }
  }
  delete() {
    for (const key of Array.from(mockFs.store.keys())) {
      if (key === this.uri || key.startsWith(this.uri + '/')) mockFs.store.delete(key);
    }
  }
}

class MockFile {
  uri: string;
  constructor(...parts: (string | MockDirectory | MockFile)[]) {
    this.uri = normalize(parts.map((p) => (typeof p === 'string' ? p : p.uri)).join('/'));
  }
  get exists(): boolean {
    return mockFs.store.get(this.uri)?.kind === 'file';
  }
  get size(): number {
    return mockFs.store.get(this.uri)?.bytes.length ?? 0;
  }
  create(_options?: { intermediates?: boolean; overwrite?: boolean }) {
    mockFs.store.set(this.uri, { kind: 'file', bytes: new Uint8Array() });
  }
  write(content: string) {
    mockFs.store.set(this.uri, { kind: 'file', bytes: new TextEncoder().encode(content) });
  }
  textSync(): string {
    const entry = mockFs.store.get(this.uri);
    if (!entry) throw new Error('File does not exist');
    return new TextDecoder().decode(entry.bytes);
  }
  delete() {
    const entry = mockFs.store.get(this.uri);
    if (!entry) throw new Error('File does not exist');
    mockFs.store.delete(this.uri);
    // Real devices actually reclaim disk space when a file is deleted —
    // simulate that so eviction logic that re-checks availableDiskSpace
    // between deletions behaves realistically instead of evicting
    // everything in one pass.
    mockFs.availableDiskSpace += entry.bytes.length;
  }
  static downloadFileAsync = async (
    url: string,
    destination: MockDirectory | MockFile,
    options?: { headers?: Record<string, string> }
  ) => {
    mockFs.lastDownloadHeaders = options?.headers;
    const destUri =
      destination instanceof MockDirectory
        ? `${destination.uri}/${url.split('/').pop() || 'download'}`
        : destination.uri;
    mockFs.store.set(destUri, { kind: 'file', bytes: new Uint8Array(mockFs.downloadedByteLength) });
    return new MockFile(destUri);
  };
}

class MockPaths {
  static get document() {
    return new MockDirectory('file:///mock/document');
  }
  static get availableDiskSpace() {
    return mockFs.availableDiskSpace;
  }
}

export function expoFileSystemMockFactory() {
  return {
    File: MockFile,
    Directory: MockDirectory,
    Paths: MockPaths,
  };
}
