// A small fake of expo-audio's real class-based API (AudioPlayer via
// createAudioPlayer, playbackStatusUpdate events, setActiveForLockScreen)
// for tests — modeled on the actual installed type definitions
// (node_modules/expo-audio/build/AudioModule.types.d.ts), not a guess.
// Follows this codebase's per-module vi.mock convention (see
// mockExpoFileSystem.ts).

export interface MockAudioStatus {
  currentTime: number;
  duration: number;
  playing: boolean;
  didJustFinish: boolean;
  isLoaded: boolean;
}

type Listener = (status: MockAudioStatus) => void;

export class MockAudioPlayer {
  playing = false;
  currentTime = 0;
  duration = 0;
  source: { uri: string } | string | null = null;
  lockScreenActive = false;
  lockScreenMetadata: Record<string, string> | undefined;

  private listeners = new Set<Listener>();

  private emit(overrides: Partial<MockAudioStatus> = {}) {
    const status: MockAudioStatus = {
      currentTime: this.currentTime,
      duration: this.duration,
      playing: this.playing,
      didJustFinish: false,
      isLoaded: true,
      ...overrides,
    };
    this.listeners.forEach((l) => l(status));
  }

  addListener(_event: 'playbackStatusUpdate', listener: Listener) {
    this.listeners.add(listener);
    return { remove: () => this.listeners.delete(listener) };
  }

  replace(source: { uri: string } | string) {
    this.source = source;
    this.currentTime = 0;
    this.duration = mockAudio.durationForSource(source);
  }

  play() {
    this.playing = true;
    this.emit();
  }

  pause() {
    this.playing = false;
    this.emit();
  }

  async seekTo(seconds: number) {
    this.currentTime = seconds;
    this.emit();
  }

  setActiveForLockScreen(active: boolean, metadata?: Record<string, string>) {
    this.lockScreenActive = active;
    this.lockScreenMetadata = metadata;
  }

  updateLockScreenMetadata(metadata: Record<string, string>) {
    this.lockScreenMetadata = metadata;
  }

  clearLockScreenControls() {
    this.lockScreenActive = false;
    this.lockScreenMetadata = undefined;
  }

  remove() {
    this.listeners.clear();
  }

  // Test-only helper: simulate the track reaching its natural end.
  simulateFinish() {
    this.playing = false;
    this.emit({ didJustFinish: true, playing: false, currentTime: this.duration });
  }
}

export const mockAudio = {
  instances: [] as MockAudioPlayer[],
  // Lets a test control what "duration" a given source reports once
  // loaded, the same way a real file's metadata would determine it.
  durationBySource: new Map<string, number>(),
  durationForSource(source: { uri: string } | string | null): number {
    const uri = typeof source === 'string' ? source : source?.uri;
    return uri ? this.durationBySource.get(uri) ?? 0 : 0;
  },
};

export function resetMockAudio() {
  mockAudio.instances = [];
  mockAudio.durationBySource.clear();
}

export function expoAudioMockFactory() {
  return {
    createAudioPlayer: (source?: { uri: string } | string | null) => {
      const player = new MockAudioPlayer();
      if (source) player.replace(source);
      mockAudio.instances.push(player);
      return player;
    },
    setAudioModeAsync: async () => {},
  };
}
