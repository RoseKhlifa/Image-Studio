import { Copy, Plus, RadioTower, Trash2 } from "lucide-react";
import type { UpstreamProfile } from "../../../types/domain";
import { upstreamProviderLabel } from "../../../lib/profiles";

export function AndroidUpstreamProfileRail({
  profiles,
  selectedId,
  activeProfileId,
  onCreate,
  onDuplicate,
  onDelete,
  onSelect,
}: {
  profiles: UpstreamProfile[];
  selectedId: string;
  activeProfileId: string;
  onCreate: () => void | Promise<void>;
  onDuplicate: () => void | Promise<void>;
  onDelete: () => void | Promise<void>;
  onSelect: (id: string) => void;
}) {
  return (
    <section className="android-upstream-profiles" aria-label="上游配置列表">
      <div className="android-upstream-section-head">
        <span>配置组</span>
        <div className="android-upstream-icon-actions">
          <button type="button" onClick={onCreate} title="新建配置">
            <Plus className="h-4 w-4" />
          </button>
          <button type="button" onClick={onDuplicate} disabled={!selectedId} title="复制当前配置">
            <Copy className="h-4 w-4" />
          </button>
          <button type="button" onClick={onDelete} disabled={!selectedId} className="danger" title="删除当前配置">
            <Trash2 className="h-4 w-4" />
          </button>
        </div>
      </div>
      <div className="android-upstream-profile-list">
        {profiles.map((profile) => {
          const selected = profile.id === selectedId;
          const active = profile.id === activeProfileId;
          return (
            <button
              key={profile.id}
              type="button"
              className={`android-upstream-profile-item ${selected ? "selected" : ""}`}
              onClick={() => onSelect(profile.id)}
              aria-current={selected ? "true" : undefined}
            >
              <span className={`android-upstream-profile-dot ${active ? "active" : ""}`} />
              <span className="android-upstream-profile-main">
                <strong>{profile.name || "未命名配置"}</strong>
                <small>
                  {upstreamProviderLabel(profile.provider ?? "openai")}
                  {profile.baseURL ? ` · ${profile.baseURL}` : " · 未填写地址"}
                </small>
              </span>
              <span className="android-upstream-profile-mode">
                <RadioTower className="h-3.5 w-3.5" />
              </span>
            </button>
          );
        })}
      </div>
    </section>
  );
}
