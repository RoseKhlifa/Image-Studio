import { Dices, X } from "lucide-react";
import type { BackgroundValue, ImageStyleValue, InputFidelityValue, ModerationValue, OutputFormatValue } from "../../types/domain";
import { BACKGROUND_OPTIONS, IMAGE_STYLE_OPTIONS, INPUT_FIDELITY_OPTIONS, MODERATION_OPTIONS, OUTPUT_FORMAT_OPTIONS } from "../../types/domain";
import { Modal } from "../../components/common/Modal";
import { vibrateForPlatform } from "./bridge";
import { useStudioStore } from "../../state/studioStore";
import { GifAnimationControls } from "../../components/panel/GifAnimationControls";

export function AndroidAdvancedSection({
  advancedOpen,
  background,
  imageStyle,
  inputFidelity,
  moderation,
  negativePrompt,
  outputCompression,
  outputFormat,
  partialImages,
  seed,
  userIdentifier,
  setAdvancedOpen,
  setField,
  surface = "phone",
}: {
  advancedOpen: boolean;
  background: BackgroundValue;
  imageStyle: ImageStyleValue;
  inputFidelity: InputFidelityValue;
  moderation: ModerationValue;
  negativePrompt: string;
  outputCompression: number;
  outputFormat: OutputFormatValue;
  partialImages: number;
  seed: number;
  userIdentifier: string;
  setAdvancedOpen: React.Dispatch<React.SetStateAction<boolean>>;
  setField: (key: string, value: any) => void;
  surface?: "phone" | "pad";
}) {
  const frameCount = useStudioStore((state) => state.frameCount);
  const frameDurationMs = useStudioStore((state) => state.frameDurationMs);
  const openAdvanced = () => {
    vibrateForPlatform(8);
    setAdvancedOpen(true);
  };
  const negativeState = negativePrompt.trim() ? "已填写" : "未填写";
  const outputFormatLabel = OUTPUT_FORMAT_OPTIONS.find((item) => item.value === outputFormat)?.label ?? outputFormat;
  const backgroundLabel = background === "transparent" ? "透明" : background === "opaque" ? "纯色" : "自动";
  const inputFidelityLabel = inputFidelity === "auto" ? "默认" : inputFidelity;
  const imageStyleLabel = imageStyle === "default" ? "默认" : imageStyle;
  const outputCompressionLabel = outputFormat === "png" || outputFormat === "gif" || outputFormat === "svg" ? "N/A" : String(outputCompression);
  const moderationLabel = moderation === "auto" ? "auto" : "low";
  const userIdentifierLabel = userIdentifier.trim() ? "已填写" : "未填写";
  const partialImagesLabel = outputFormat === "gif" || outputFormat === "svg" || partialImages === 0 ? "仅最终结果" : `${partialImages} 帧`;
  const title = outputFormat === "gif" ? "GIF 动画与高级设置" : surface === "pad" ? "10 项高级设置" : "负向提示词、背景、压缩、输入保真、风格、审核、用户标识、预览帧数、Seed 与输出格式";
  const negativeLabel = surface === "pad" ? "负向" : "负向提示词";

  return (
    <section className={`android-advanced-block ${surface === "pad" ? "android-pad-advanced-block" : ""}`}>
      <button
        type="button"
        onClick={openAdvanced}
        className="platform-card android-advanced-toggle"
      >
        <span>
          <span className="android-phone-kicker !mb-0">高级参数</span>
          <strong>{title}</strong>
          <span className="android-advanced-summary-grid">
            <span>
              <span>{negativeLabel}</span>
              <strong>{negativeState}</strong>
            </span>
            <span>
              <span>输出格式</span>
              <strong>{outputFormatLabel}</strong>
            </span>
            <span>
              <span>审核</span>
              <strong>{moderationLabel}</strong>
            </span>
            <span>
              <span>{outputFormat === "gif" ? "动画帧数" : "背景"}</span>
              <strong>{outputFormat === "gif" ? `${frameCount} 帧` : backgroundLabel}</strong>
            </span>
            <span>
              <span>{outputFormat === "gif" ? "每帧时长" : "压缩"}</span>
              <strong>{outputFormat === "gif" ? `${frameDurationMs} ms` : outputCompressionLabel}</strong>
            </span>
            <span>
              <span>保真</span>
              <strong>{inputFidelityLabel}</strong>
            </span>
            <span>
              <span>风格</span>
              <strong>{imageStyleLabel}</strong>
            </span>
            <span>
              <span>标识</span>
              <strong>{userIdentifierLabel}</strong>
            </span>
            <span>
              <span>预览</span>
              <strong>{partialImagesLabel}</strong>
            </span>
            <span>
              <span>Seed</span>
              <strong>{seed > 0 ? seed : "随机"}</strong>
            </span>
          </span>
        </span>
        <span className="android-advanced-toggle-state">编辑</span>
      </button>

      <Modal
        open={advancedOpen}
        onClose={() => setAdvancedOpen(false)}
        title="高级参数"
        width={680}
      >
        <AndroidAdvancedEditor
          background={background}
          imageStyle={imageStyle}
          inputFidelity={inputFidelity}
          moderation={moderation}
          negativePrompt={negativePrompt}
          outputCompression={outputCompression}
          outputFormat={outputFormat}
          partialImages={partialImages}
          seed={seed}
          userIdentifier={userIdentifier}
          setField={setField}
        />
      </Modal>
    </section>
  );
}

function AndroidAdvancedEditor({
  background,
  imageStyle,
  inputFidelity,
  moderation,
  negativePrompt,
  outputCompression,
  outputFormat,
  partialImages,
  seed,
  userIdentifier,
  setField,
}: {
  background: BackgroundValue;
  imageStyle: ImageStyleValue;
  inputFidelity: InputFidelityValue;
  moderation: ModerationValue;
  negativePrompt: string;
  outputCompression: number;
  outputFormat: OutputFormatValue;
  partialImages: number;
  seed: number;
  userIdentifier: string;
  setField: (key: string, value: any) => void;
}) {
  return (
    <div className="android-advanced-modal-panel">
      <div className="android-phone-advanced-section">
        <div className="android-phone-advanced-label">负向提示词</div>
        <textarea
          value={negativePrompt}
          placeholder="不希望出现的元素"
          onChange={(event) => setField("negativePrompt", event.target.value)}
          className="focus-ring android-phone-advanced-textarea"
        />
      </div>

      <div className="android-phone-advanced-section">
        <div className="android-phone-advanced-label">输出格式</div>
        <div className="android-phone-format-row">
          {OUTPUT_FORMAT_OPTIONS.map((item) => (
            <button
              key={item.value}
              type="button"
              onClick={() => {
                vibrateForPlatform(5);
                setField("outputFormat", item.value as OutputFormatValue);
              }}
              className={`android-choice-chip ${outputFormat === item.value ? "active" : ""}`}
            >
              {item.label}
            </button>
          ))}
        </div>
      </div>

      {outputFormat === "gif" ? (
        <div className="android-phone-advanced-section">
          <div className="android-phone-advanced-label">GIF 动画</div>
          <GifAnimationControls variant="android" />
        </div>
      ) : null}

      {outputFormat !== "gif" && outputFormat !== "svg" ? <>
        <div className="android-phone-advanced-section">
          <div className="android-phone-advanced-label">背景</div>
          <div className="android-phone-format-row">
            {BACKGROUND_OPTIONS.map((item) => (
              <button
                key={item.value}
                type="button"
                onClick={() => {
                  vibrateForPlatform(5);
                  setField("background", item.value as BackgroundValue);
                }}
                className={`android-choice-chip ${background === item.value ? "active" : ""}`}
              >
                {item.label}
              </button>
            ))}
          </div>
        </div>

        <div className="android-phone-advanced-section">
          <div className="android-phone-advanced-label">输出压缩</div>
          <input
            type="number"
            value={outputCompression}
            min={0}
            max={100}
            step={1}
            onChange={(event) => {
              const raw = event.target.value.trim();
              setField("outputCompression", raw ? Math.max(0, Math.min(100, Math.round(Number(raw) || 0))) : 100);
            }}
            className="focus-ring android-phone-seed-input font-mono-token"
          />
        </div>

      </> : null}

      <div className="android-phone-advanced-section">
        <div className="android-phone-advanced-label">输入保真</div>
        <div className="android-phone-format-row">
          {INPUT_FIDELITY_OPTIONS.map((item) => (
            <button
              key={item.value}
              type="button"
              onClick={() => {
                vibrateForPlatform(5);
                setField("inputFidelity", item.value as InputFidelityValue);
              }}
              className={`android-choice-chip ${inputFidelity === item.value ? "active" : ""}`}
            >
              {item.label}
            </button>
          ))}
        </div>
      </div>

      <div className="android-phone-advanced-section">
        <div className="android-phone-advanced-label">图像风格</div>
        <div className="android-phone-format-row">
          {IMAGE_STYLE_OPTIONS.map((item) => (
            <button
              key={item.value}
              type="button"
              onClick={() => {
                vibrateForPlatform(5);
                setField("imageStyle", item.value as ImageStyleValue);
              }}
              className={`android-choice-chip ${imageStyle === item.value ? "active" : ""}`}
            >
              {item.label}
            </button>
          ))}
        </div>
      </div>

      <div className="android-phone-advanced-section">
        <div className="android-phone-advanced-label">内容审核</div>
        <div className="android-phone-format-row">
          {MODERATION_OPTIONS.map((item) => (
            <button
              key={item.value}
              type="button"
              onClick={() => {
                vibrateForPlatform(5);
                setField("moderation", item.value as ModerationValue);
              }}
              className={`android-choice-chip ${moderation === item.value ? "active" : ""}`}
            >
              {item.label}
            </button>
          ))}
        </div>
      </div>

      <div className="android-phone-advanced-section">
        <div className="android-phone-advanced-label">稳定用户标识</div>
        <input
          type="text"
          value={userIdentifier}
          maxLength={64}
          placeholder="建议填哈希后的用户标识"
          onChange={(event) => setField("userIdentifier", event.target.value)}
          className="focus-ring android-phone-seed-input font-mono-token"
        />
      </div>

      {outputFormat !== "gif" && outputFormat !== "svg" ? <div className="android-phone-advanced-section">
        <div className="android-phone-advanced-label">流式预览帧数</div>
        <div className="android-phone-format-row">
          {[0, 1, 2, 3].map((value) => (
            <button
              key={value}
              type="button"
              onClick={() => {
                vibrateForPlatform(5);
                setField("partialImages", value);
              }}
              className={`android-choice-chip ${partialImages === value ? "active" : ""}`}
            >
              {value === 0 ? "仅最终图" : `${value} 帧`}
            </button>
          ))}
        </div>
        <p className="android-parameter-note mt-2">高并发或 2K/4K 大尺寸任务时，应用可能自动关闭流式预览，以优先保证最终图完整。</p>
      </div> : null}

      <div className="android-phone-advanced-section">
        <div className="android-phone-advanced-label">Seed</div>
        <div className="android-phone-seed-row">
          <input
            type="number"
            value={seed || ""}
            placeholder="留空为随机"
            min={0}
            onChange={(event) => setField("seed", Number(event.target.value) || 0)}
            className="focus-ring android-phone-seed-input font-mono-token"
          />
          <button
            type="button"
            onClick={() => {
              vibrateForPlatform(5);
              setField("seed", Math.floor(Math.random() * 2_000_000_000));
            }}
            title="随机 seed"
            className="platform-action-btn android-phone-seed-icon-button"
          >
            <Dices className="h-3.5 w-3.5" />
          </button>
          {seed > 0 ? (
            <button
              type="button"
              onClick={() => {
                vibrateForPlatform(5);
                setField("seed", 0);
              }}
              title="清除"
              className="platform-action-btn android-phone-seed-icon-button danger"
            >
              <X className="h-3.5 w-3.5" />
            </button>
          ) : null}
        </div>
      </div>
    </div>
  );
}
