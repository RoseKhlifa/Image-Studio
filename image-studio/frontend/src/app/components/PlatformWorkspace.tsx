import { lazy, Suspense } from "react";
import { usePlatform } from "../../platform/context";
import type { AndroidView } from "../../platform/types";

const AndroidShell = lazy(() => import("../../platform/android/AndroidShell").then((module) => ({ default: module.AndroidShell })));
const DesktopShell = lazy(() => import("../../platform/desktop/DesktopShell").then((module) => ({ default: module.DesktopShell })));

function ShellFallback({
  fullscreen,
  isAndroid,
  isAndroidPad,
}: {
  fullscreen: boolean;
  isAndroid: boolean;
  isAndroidPad: boolean;
}) {
  const classes = ["studio"];
  if (fullscreen) classes.push("fullscreen");
  if (isAndroid) classes.push(isAndroidPad ? "android-pad" : "android-phone");
  return <div className={classes.join(" ")} aria-hidden="true" />;
}

export function PlatformWorkspace({
  fullscreen,
  androidView,
  onChangeAndroidView,
}: {
  fullscreen: boolean;
  androidView: AndroidView;
  onChangeAndroidView: (value: AndroidView) => void;
}) {
  const { isAndroid, isAndroidPad, androidWidthClass } = usePlatform();

  return (
    <Suspense fallback={<ShellFallback fullscreen={fullscreen} isAndroid={isAndroid} isAndroidPad={isAndroidPad} />}>
      {isAndroid ? (
        <AndroidShell
          fullscreen={fullscreen}
          isPad={isAndroidPad}
          isExpandedPad={androidWidthClass === "expanded"}
          androidView={androidView}
          onChangeView={onChangeAndroidView}
        />
      ) : (
        <DesktopShell fullscreen={fullscreen} />
      )}
    </Suspense>
  );
}
