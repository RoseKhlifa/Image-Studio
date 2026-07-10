# Android Shell

This module packages the existing React frontend into a single Android APK.
The frontend always builds the `android` target, and the app switches between
phone and pad shells at runtime based on the current window size / orientation.

The shell is a minimal WebView wrapper. During Gradle asset merging it runs the
frontend build for the matching target and copies `image-studio/frontend/dist/`
into `app/src/main/assets/web/`.

Current scope:

- APK packaging works from the WebView shell
- Frontend startup is supported by the Android-side `AndroidImageStudio` bridge
- Active image generation and editing requests use a `dataSync` foreground
  service, a low-priority notification, and a bounded partial wake lock so a
  user-started request can continue after the app moves to the background
- Desktop-only backend features that still depend on the Go/Wails runtime are
  surfaced as explicit "not implemented in Android shell yet" errors

Background execution is scoped to active image requests. The service starts
when the native HTTP or Responses WebSocket request begins, tracks concurrent
requests, and stops after the final request completes, fails, or is cancelled.
Android 13 and newer ask for notification permission when the first image task
starts. If that permission is denied, Android can still expose the foreground
service in its active-apps UI, but it may omit the notification from the drawer.

The shell does not request overlay permission and does not use a transparent or
one-pixel window to keep an idle process alive. Force-stop, network loss, and
device-specific battery policies can still terminate an active task.

Local build:

```bash
cd android-shell
./gradlew assembleRelease
```

Local verification without a connected device:

```bash
cd ..
node scripts/verify-local-android-shell.mjs
```

This script assembles the debug APK, checks `versionName` / `versionCode`,
verifies the APK signature, confirms the built frontend assets are embedded
into the package, and runs the Android JVM unit tests for shell-side parsing
logic.

If you already have a device or emulator attached over `adb`, you can also ask
the script to install and launch the debug APK:

```bash
IMAGE_STUDIO_ANDROID_DEVICE_SMOKE=1 node scripts/verify-local-android-shell.mjs
```

Optionally pin a specific device:

```bash
IMAGE_STUDIO_ANDROID_DEVICE_SMOKE=1 IMAGE_STUDIO_ANDROID_SERIAL=<serial> node scripts/verify-local-android-shell.mjs
```

MuMu emulator debugging:

- See `../docs/mumu-android-debug.md` for the shared ADB connection,
  Docker build, install, screenshot, rotation, and troubleshooting workflow.
