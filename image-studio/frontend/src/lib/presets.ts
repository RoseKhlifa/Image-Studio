import type {
  AutoAspectResolutionPreset,
  BackgroundValue,
  ImageStyleValue,
  InputFidelityValue,
  ModerationValue,
  OutputFormatValue,
  Preset,
  QualityValue,
  SizeValue,
} from "../types/domain";

export type PresetEditableFields = Pick<Preset, "name" | "size" | "quality" | "outputFormat" | "batchCount" | "styleTag">;

export type PresetStateSnapshot = {
  size: SizeValue;
  quality: QualityValue;
  outputFormat: OutputFormatValue;
  negativePrompt: string;
  background: BackgroundValue;
  outputCompression: number;
  inputFidelity: InputFidelityValue;
  imageStyle: ImageStyleValue;
  moderation: ModerationValue;
  batchCount: number;
  styleTag: string;
  editAutoAspectResolution: AutoAspectResolutionPreset;
};

export function pickPresetStateSnapshot(source: PresetStateSnapshot): PresetStateSnapshot {
  return {
    size: source.size,
    quality: source.quality,
    outputFormat: source.outputFormat,
    negativePrompt: source.negativePrompt,
    background: source.background,
    outputCompression: source.outputCompression,
    inputFidelity: source.inputFidelity,
    imageStyle: source.imageStyle,
    moderation: source.moderation,
    batchCount: source.batchCount,
    styleTag: source.styleTag,
    editAutoAspectResolution: source.editAutoAspectResolution,
  };
}

export function buildPresetFromSnapshot(name: string, id: string, source: PresetStateSnapshot): Preset {
  return {
    id,
    name: name.trim(),
    ...pickPresetStateSnapshot(source),
  };
}

export function buildPresetPatch(preset: Preset, current: PresetStateSnapshot): PresetStateSnapshot {
  return {
    size: preset.size,
    quality: preset.quality,
    outputFormat: preset.outputFormat ?? current.outputFormat,
    negativePrompt: preset.negativePrompt,
    background: preset.background ?? current.background,
    outputCompression: preset.outputCompression ?? current.outputCompression,
    inputFidelity: preset.inputFidelity ?? current.inputFidelity,
    imageStyle: preset.imageStyle ?? current.imageStyle,
    moderation: preset.moderation ?? current.moderation,
    batchCount: preset.batchCount,
    styleTag: preset.styleTag ?? current.styleTag,
    editAutoAspectResolution: preset.editAutoAspectResolution ?? "",
  };
}

export function presetMatchesSnapshot(preset: Preset, current: PresetStateSnapshot): boolean {
  const patch = buildPresetPatch(preset, current);
  return (
    patch.size === current.size
    && patch.quality === current.quality
    && patch.outputFormat === current.outputFormat
    && patch.negativePrompt === current.negativePrompt
    && patch.background === current.background
    && patch.outputCompression === current.outputCompression
    && patch.inputFidelity === current.inputFidelity
    && patch.imageStyle === current.imageStyle
    && patch.moderation === current.moderation
    && patch.batchCount === current.batchCount
    && patch.styleTag === current.styleTag
    && patch.editAutoAspectResolution === current.editAutoAspectResolution
  );
}

export function findMatchingPresetId(presets: Preset[], current: PresetStateSnapshot): string | null {
  return presets.find((preset) => presetMatchesSnapshot(preset, current))?.id ?? null;
}

export function normalizeSelectedPresetId(presets: Preset[], selectedPresetId: string | null): string | null {
  if (!selectedPresetId) return null;
  return presets.some((preset) => preset.id === selectedPresetId) ? selectedPresetId : null;
}

export function nextDefaultPresetName(presets: Preset[] = []): string {
  const usedNumbers = new Set<number>();
  for (const preset of presets) {
    const match = preset.name.trim().match(/^配置\s*(\d+)$/);
    if (!match) continue;
    const value = Number(match[1]);
    if (Number.isInteger(value) && value > 0) usedNumbers.add(value);
  }
  let index = 1;
  while (usedNumbers.has(index)) index += 1;
  return `配置${index}`;
}
