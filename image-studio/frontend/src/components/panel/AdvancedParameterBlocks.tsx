import { Dices, X } from "lucide-react";
import type { BackgroundValue, ImageStyleValue, InputFidelityValue, ModerationValue, OutputFormatValue } from "../../types/domain";
import { BACKGROUND_OPTIONS, IMAGE_STYLE_OPTIONS, INPUT_FIDELITY_OPTIONS, MODERATION_OPTIONS, OUTPUT_FORMAT_OPTIONS } from "../../types/domain";

type SegRenderer = (props: { children: React.ReactNode }) => React.ReactNode;
type SegItemRenderer = (props: {
  active: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) => React.ReactNode;

export function AdvancedCard({
  title,
  hint,
  children,
  variant = "desktop",
  className = "",
}: {
  title: string;
  hint?: string;
  children: React.ReactNode;
  variant?: "mac" | "desktop";
  className?: string;
}) {
  const isMac = variant === "mac";

  return (
    <section
      className={`min-w-0 ${
        isMac
          ? "rounded-[18px] border border-black/[0.06] bg-white/55 px-3.5 py-3.5 ring-1 ring-black/[0.02] dark:border-white/[0.07] dark:bg-white/[0.035] dark:ring-white/[0.03]"
          : "rounded-[20px] border border-white/12 bg-[var(--surface)]/70 px-4 py-4 ring-1 ring-black/[0.03] dark:ring-white/[0.04]"
      } ${className}`}
    >
      <div className={`${isMac ? "text-[12px]" : "text-[11px]"} font-semibold uppercase tracking-[0.12em] text-zinc-500 dark:text-zinc-300`}>
        {title}
      </div>
      {hint ? (
        <p className={`${isMac ? "mt-1.5 text-[12px] leading-[1.65]" : "mt-1.5 text-[12px] leading-6"} text-zinc-500 dark:text-zinc-400`}>
          {hint}
        </p>
      ) : null}
      <div className={isMac ? "mt-3.5" : "mt-3"}>{children}</div>
    </section>
  );
}

export function AdvancedNegativePromptField({
  negativePrompt,
  onChange,
  variant,
}: {
  negativePrompt: string;
  onChange: (value: string) => void;
  variant: "mac" | "desktop";
}) {
  return (
    <textarea
      value={negativePrompt}
      placeholder={variant === "mac"
        ? "例如：不要文字、不要水印、不要多余肢体、不要过曝"
        : "负向提示词(不希望出现的元素)..."}
      onChange={(e) => onChange(e.target.value)}
      className={`focus-ring w-full resize-y border border-black/[0.08] bg-[var(--surface)] text-zinc-900 placeholder:text-zinc-400 dark:border-white/[0.08] dark:text-zinc-100 dark:placeholder:text-zinc-500 ${
        variant === "mac"
          ? "min-h-[150px] rounded-[18px] px-4 py-3.5 text-[14px] leading-[1.72]"
          : "min-h-[84px] rounded-[16px] px-3.5 py-3 text-[13px] leading-relaxed"
      }`}
    />
  );
}

export function AdvancedOutputFormatField({
  outputFormat,
  onChange,
  Seg,
  SegItem,
  noteClassName = "text-[10px] text-zinc-500",
}: {
  outputFormat: OutputFormatValue;
  onChange: (value: OutputFormatValue) => void;
  Seg: SegRenderer;
  SegItem: SegItemRenderer;
  noteClassName?: string;
}) {
  return (
    <>
      <div className="grid gap-1.5">
        {[OUTPUT_FORMAT_OPTIONS.slice(0, 3), OUTPUT_FORMAT_OPTIONS.slice(3)].map((options, index) => (
          <Seg key={index}>
            {options.map((item) => (
              <SegItem
                key={item.value}
                active={outputFormat === item.value}
                onClick={() => onChange(item.value as OutputFormatValue)}
              >
                <span className="whitespace-nowrap">{item.label}</span>
              </SegItem>
            ))}
          </Seg>
        ))}
      </div>
      <p className={`mt-1 ${noteClassName}`}>JPEG/WebP 体积更小；GIF 生成动画；SVG 生成可编辑矢量。GIF/SVG 需要 GPT Image 2.6 Images API。</p>
    </>
  );
}

export function AdvancedModerationField({
  moderation,
  onChange,
  Seg,
  SegItem,
  noteClassName = "text-[10px] text-zinc-500",
}: {
  moderation: ModerationValue;
  onChange: (value: ModerationValue) => void;
  Seg: SegRenderer;
  SegItem: SegItemRenderer;
  noteClassName?: string;
}) {
  return (
    <>
      <Seg>
        {MODERATION_OPTIONS.map((item) => (
          <SegItem
            key={item.value}
            active={moderation === item.value}
            onClick={() => onChange(item.value as ModerationValue)}
          >
            {item.label}
          </SegItem>
        ))}
      </Seg>
      <p className={`mt-1 ${noteClassName}`}>`low` 更宽松；`auto` 使用官方默认审核强度。仅 GPT 图像模型支持。</p>
    </>
  );
}

export function AdvancedBackgroundField({
  background,
  onChange,
  Seg,
  SegItem,
  noteClassName = "text-[10px] text-zinc-500",
}: {
  background: BackgroundValue;
  onChange: (value: BackgroundValue) => void;
  Seg: SegRenderer;
  SegItem: SegItemRenderer;
  noteClassName?: string;
}) {
  return (
    <>
      <Seg>
        {BACKGROUND_OPTIONS.map((item) => (
          <SegItem
            key={item.value}
            active={background === item.value}
            onClick={() => onChange(item.value as BackgroundValue)}
          >
            {item.label}
          </SegItem>
        ))}
      </Seg>
      <p className={`mt-1 ${noteClassName}`}>仅 GPT 图像模型支持。`transparent` 需要 PNG/WebP；GIF/SVG 会省略栅格背景设置。</p>
    </>
  );
}

export function AdvancedOutputCompressionField({
  outputCompression,
  onChange,
  variant,
  noteClassName = "text-[10px] text-zinc-500",
}: {
  outputCompression: number;
  onChange: (value: number) => void;
  variant: "mac" | "desktop";
  noteClassName?: string;
}) {
  const inputClassName = variant === "mac"
    ? "min-h-[44px] rounded-[18px] px-4 py-3 text-[14px]"
    : "min-h-[42px] rounded-[10px] px-3 py-2.5 text-[13px]";

  return (
    <div className="flex flex-col gap-2">
      <input
        type="number"
        value={outputCompression}
        min={0}
        max={100}
        step={1}
        onChange={(e) => {
          const raw = e.target.value.trim();
          if (!raw) {
            onChange(100);
            return;
          }
          onChange(Math.max(0, Math.min(100, Math.round(Number(raw) || 0))));
        }}
        className={`focus-ring w-full border border-black/[0.08] bg-[var(--surface)] font-mono-token text-zinc-900 placeholder:text-zinc-400 dark:border-white/[0.08] dark:text-zinc-100 dark:placeholder:text-zinc-500 ${inputClassName}`}
      />
      <p className={noteClassName}>仅 JPEG/WebP 生效，范围 `0-100`，默认 `100`。</p>
    </div>
  );
}

export function AdvancedInputFidelityField({
  inputFidelity,
  onChange,
  Seg,
  SegItem,
  noteClassName = "text-[10px] text-zinc-500",
}: {
  inputFidelity: InputFidelityValue;
  onChange: (value: InputFidelityValue) => void;
  Seg: SegRenderer;
  SegItem: SegItemRenderer;
  noteClassName?: string;
}) {
  return (
    <>
      <Seg>
        {INPUT_FIDELITY_OPTIONS.map((item) => (
          <SegItem
            key={item.value}
            active={inputFidelity === item.value}
            onClick={() => onChange(item.value as InputFidelityValue)}
          >
            {item.label}
          </SegItem>
        ))}
      </Seg>
      <p className={`mt-1 ${noteClassName}`}>用于图生图/参考图流程。GPT Image 2.x 会自动高保真并忽略此项。</p>
    </>
  );
}

export function AdvancedImageStyleField({
  imageStyle,
  onChange,
  Seg,
  SegItem,
  noteClassName = "text-[10px] text-zinc-500",
}: {
  imageStyle: ImageStyleValue;
  onChange: (value: ImageStyleValue) => void;
  Seg: SegRenderer;
  SegItem: SegItemRenderer;
  noteClassName?: string;
}) {
  return (
    <>
      <Seg>
        {IMAGE_STYLE_OPTIONS.map((item) => (
          <SegItem
            key={item.value}
            active={imageStyle === item.value}
            onClick={() => onChange(item.value as ImageStyleValue)}
          >
            {item.label}
          </SegItem>
        ))}
      </Seg>
      <p className={`mt-1 ${noteClassName}`}>仅 `dall-e-3` 文生图支持；默认值会省略该字段。</p>
    </>
  );
}

export function AdvancedUserIdentifierField({
  userIdentifier,
  onChange,
  variant,
  noteClassName = "text-[10px] text-zinc-500",
}: {
  userIdentifier: string;
  onChange: (value: string) => void;
  variant: "mac" | "desktop";
  noteClassName?: string;
}) {
  const inputClassName = variant === "mac"
    ? "min-h-[44px] rounded-[18px] px-4 py-3 text-[14px]"
    : "min-h-[42px] rounded-[10px] px-3 py-2.5 text-[13px]";

  return (
    <div className="flex flex-col gap-2">
      <input
        type="text"
        value={userIdentifier}
        maxLength={64}
        placeholder={variant === "mac" ? "建议填哈希后的用户标识" : "稳定用户标识(建议用哈希值)"}
        onChange={(e) => onChange(e.target.value)}
        className={`focus-ring w-full border border-black/[0.08] bg-[var(--surface)] font-mono-token text-zinc-900 placeholder:text-zinc-400 dark:border-white/[0.08] dark:text-zinc-100 dark:placeholder:text-zinc-500 ${inputClassName}`}
      />
      <p className={noteClassName}>Responses 会发 `safety_identifier`，Images API 会发 `user`。建议传哈希值，最长 `64` 字符。</p>
    </div>
  );
}

export function AdvancedPartialImagesField({
  partialImages,
  onChange,
  Seg,
  SegItem,
  noteClassName = "text-[10px] text-zinc-500",
}: {
  partialImages: number;
  onChange: (value: number) => void;
  Seg: SegRenderer;
  SegItem: SegItemRenderer;
  noteClassName?: string;
}) {
  return (
    <>
      <Seg>
        {[0, 1, 2, 3].map((value) => (
          <SegItem
            key={value}
            active={partialImages === value}
            onClick={() => onChange(value)}
          >
            {value === 0 ? "仅最终图" : `${value} 帧`}
          </SegItem>
        ))}
      </Seg>
      <p className={`mt-1 ${noteClassName}`}>官方 `partial_images` 范围 `0-3`。`0` 只返回最终图，`1-3` 会流式返回预览帧。高并发批量生成时，应用可能自动关闭预览以优先保证最终图完整。</p>
    </>
  );
}

export function AdvancedSeedField({
  seed,
  onChange,
  onRandomize,
  onClear,
  variant,
}: {
  seed: number;
  onChange: (value: number) => void;
  onRandomize: () => void;
  onClear: () => void;
  variant: "mac" | "desktop";
}) {
  const inputClassName = variant === "mac"
    ? "min-h-[44px] rounded-[18px] px-4 py-3 text-[14px]"
    : "min-h-[42px] rounded-[10px] px-3 py-2.5 text-[13px]";
  const buttonShape = variant === "mac" ? "rounded-full" : "rounded-[8px]";

  return (
    <div className="flex flex-col gap-2.5">
      <input
        type="number"
        value={seed || ""}
        placeholder={variant === "mac" ? "留空为随机" : "seed (留空=随机)"}
        min={0}
        onChange={(e) => onChange(Number(e.target.value) || 0)}
        className={`focus-ring w-full border border-black/[0.08] bg-[var(--surface)] font-mono-token text-zinc-900 placeholder:text-zinc-400 dark:border-white/[0.08] dark:text-zinc-100 dark:placeholder:text-zinc-500 ${inputClassName}`}
      />
      <div className="grid grid-cols-2 gap-2">
        <button
          onClick={onRandomize}
          title="随机 seed"
          type="button"
          className={`platform-action-btn inline-flex min-h-[40px] min-w-0 items-center justify-center gap-1.5 whitespace-nowrap border border-black/[0.08] px-3 py-2 text-[12px] font-medium text-zinc-700 transition-colors hover:border-[color:var(--accent)]/35 hover:text-[var(--accent)] dark:border-white/[0.08] dark:text-zinc-300 ${buttonShape}`}
        >
          <Dices className="h-3.5 w-3.5" /> 随机
        </button>
        <button
          onClick={onClear}
          title="清除"
          disabled={seed <= 0}
          type="button"
          className={`platform-action-btn inline-flex min-h-[40px] min-w-0 items-center justify-center gap-1.5 whitespace-nowrap border border-black/[0.08] px-3 py-2 text-[12px] font-medium text-zinc-500 transition-colors hover:border-red-400/40 hover:text-red-400 disabled:cursor-not-allowed disabled:opacity-45 dark:border-white/[0.08] dark:text-zinc-300 ${buttonShape}`}
        >
          <X className="h-3.5 w-3.5" /> 清空
        </button>
      </div>
    </div>
  );
}
