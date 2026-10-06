import { Boxes, ClipboardPaste, Plus, RadioTower } from "lucide-react";
import type { UpstreamProvider } from "../../../types/domain";
import { ANDROID_PROVIDER_OPTIONS } from "./useAndroidUpstreamConfig";

export function AndroidUpstreamEmptyState({
  onCreate,
  onQuickImport,
}: {
  onCreate: (provider: UpstreamProvider) => void | Promise<void>;
  onQuickImport: () => void | Promise<void>;
}) {
  return (
    <section className="android-upstream-empty">
      <div className="android-upstream-empty-icon">
        <RadioTower className="h-5 w-5" />
      </div>
      <div className="android-upstream-empty-copy">
        <h4>添加第一个上游</h4>
        <p>保存中转站根地址和 API Key 后，可分别指定生图渠道与 AI 优化 / 图片反推渠道。</p>
      </div>
      <div className="android-upstream-create-grid">
        <button type="button" onClick={() => void onQuickImport()} className="android-upstream-quick-import-card">
          <span className="android-upstream-create-icon">
            <ClipboardPaste className="h-4 w-4" />
          </span>
          <span>
            <strong>粘贴 JSON 快捷导入</strong>
            <small>支持 `newapi_channel_conn`、OpenCode `provider` 和本应用导出文件。</small>
          </span>
          <Plus className="h-4 w-4" />
        </button>
        {ANDROID_PROVIDER_OPTIONS.map((option) => (
          <button key={option.id} type="button" onClick={() => onCreate(option.id)}>
            <span className="android-upstream-create-icon">
              {option.id === "openai" ? <RadioTower className="h-4 w-4" /> : <Boxes className="h-4 w-4" />}
            </span>
            <span>
              <strong>{option.title}</strong>
              <small>{option.meta}</small>
            </span>
            <Plus className="h-4 w-4" />
          </button>
        ))}
      </div>
    </section>
  );
}
