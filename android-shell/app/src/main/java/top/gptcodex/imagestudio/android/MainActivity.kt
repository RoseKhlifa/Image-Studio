package top.gptcodex.imagestudio.android

import android.annotation.SuppressLint
import android.Manifest
import android.content.ActivityNotFoundException
import android.content.Intent
import android.content.pm.PackageManager
import android.content.res.Configuration
import android.net.Uri
import android.os.Build
import android.os.Bundle
import android.provider.MediaStore
import android.util.Log
import android.view.View
import android.webkit.ConsoleMessage
import android.webkit.WebChromeClient
import android.webkit.WebResourceRequest
import android.webkit.WebResourceResponse
import android.webkit.WebSettings
import android.webkit.WebView
import androidx.activity.enableEdgeToEdge
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AppCompatActivity
import androidx.core.content.ContextCompat
import androidx.core.view.ViewCompat
import androidx.core.view.WindowInsetsCompat
import androidx.webkit.WebSettingsCompat
import androidx.webkit.WebViewAssetLoader
import androidx.webkit.WebViewFeature
import androidx.webkit.WebViewClientCompat

class MainActivity : AppCompatActivity() {
    private lateinit var webView: WebView
    private lateinit var bridge: AndroidImageStudioBridge
    private lateinit var assetLoader: WebViewAssetLoader
    private val openImageDocumentLauncher = registerForActivityResult(ActivityResultContracts.OpenDocument()) { uri: Uri? ->
        if (::bridge.isInitialized) bridge.onOpenImageDialogResult(uri)
    }
    private val openImagePickerLauncher = registerForActivityResult(ActivityResultContracts.StartActivityForResult()) { result ->
        if (::bridge.isInitialized) bridge.onOpenImageDialogResult(result.data?.data)
    }
    private val requestLegacyGalleryPermissionLauncher = registerForActivityResult(ActivityResultContracts.RequestPermission()) {
        launchGalleryImagePicker()
    }
    private val requestNotificationPermissionLauncher = registerForActivityResult(ActivityResultContracts.RequestPermission()) {}
    private val importHistoryLauncher = registerForActivityResult(ActivityResultContracts.OpenDocument()) { uri: Uri? ->
        if (::bridge.isInitialized) bridge.onImportHistoryResult(uri)
    }

    private fun ensureBackgroundTaskNotificationPermission() {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.TIRAMISU) return
        runOnUiThread {
            if (isFinishing || isDestroyed) return@runOnUiThread
            if (ContextCompat.checkSelfPermission(this, Manifest.permission.POST_NOTIFICATIONS) == PackageManager.PERMISSION_GRANTED) {
                return@runOnUiThread
            }
            val prefs = getSharedPreferences(notificationPermissionPrefs, MODE_PRIVATE)
            if (prefs.getBoolean(notificationPermissionRequestedKey, false)) return@runOnUiThread
            try {
                requestNotificationPermissionLauncher.launch(Manifest.permission.POST_NOTIFICATIONS)
                prefs.edit().putBoolean(notificationPermissionRequestedKey, true).apply()
            } catch (_: IllegalStateException) {
                // The foreground service can still run; Android exposes it in the active-apps UI.
            }
        }
    }

    private fun launchImageImport() {
        if (Build.VERSION.SDK_INT <= Build.VERSION_CODES.S_V2) {
            val granted = ContextCompat.checkSelfPermission(
                this,
                Manifest.permission.READ_EXTERNAL_STORAGE,
            ) == PackageManager.PERMISSION_GRANTED
            if (!granted) {
                requestLegacyGalleryPermissionLauncher.launch(Manifest.permission.READ_EXTERNAL_STORAGE)
                return
            }
        }
        launchGalleryImagePicker()
    }

    private fun launchGalleryImagePicker() {
        val intent = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            Intent(MediaStore.ACTION_PICK_IMAGES).apply {
                type = "image/*"
            }
        } else {
            @Suppress("DEPRECATION")
            Intent(Intent.ACTION_PICK, MediaStore.Images.Media.EXTERNAL_CONTENT_URI).apply {
                type = "image/*"
            }
        }

        try {
            openImagePickerLauncher.launch(intent)
        } catch (_: ActivityNotFoundException) {
            openImageDocumentLauncher.launch(arrayOf("image/*"))
        } catch (_: IllegalStateException) {
            openImageDocumentLauncher.launch(arrayOf("image/*"))
        }
    }

    @SuppressLint("SetJavaScriptEnabled")
    override fun onCreate(savedInstanceState: Bundle?) {
        enableEdgeToEdge()
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_main)

        webView = findViewById(R.id.webview)
        
        // Apply edge-to-edge padding to the webview container if needed, 
        // but typically we want the web content to handle its own safe areas.
        // For now, we'll let it flow under the bars.
        ViewCompat.setOnApplyWindowInsetsListener(webView) { _, insets ->
            insets
        }

        bridge = AndroidImageStudioBridge(
            this,
            webView,
            launchOpenImageDialog = {
                launchImageImport()
            },
            launchImportHistory = {
                importHistoryLauncher.launch(arrayOf("application/json", "text/plain", "*/*"))
            },
            ensureBackgroundTaskNotificationPermission = {
                ensureBackgroundTaskNotificationPermission()
            },
        )
        assetLoader = WebViewAssetLoader.Builder()
            .addPathHandler("/assets/", WebViewAssetLoader.AssetsPathHandler(this))
            .build()

        webView.settings.apply {
            javaScriptEnabled = true
            domStorageEnabled = true
            allowFileAccess = true
            allowContentAccess = true
            cacheMode = WebSettings.LOAD_DEFAULT
            mixedContentMode = WebSettings.MIXED_CONTENT_ALWAYS_ALLOW
            useWideViewPort = true
            loadWithOverviewMode = true
            mediaPlaybackRequiresUserGesture = false
            databaseEnabled = true
            
            // Performance optimizations
            setSupportZoom(false)
            displayZoomControls = false
            builtInZoomControls = false
        }
        
        // Hardware acceleration
        webView.setLayerType(View.LAYER_TYPE_HARDWARE, null)

        // Keep the Android shell visually consistent with the app's own theme
        // tokens instead of letting WebView auto-darken individual form controls.
        when {
            WebViewFeature.isFeatureSupported(WebViewFeature.ALGORITHMIC_DARKENING) -> {
                WebSettingsCompat.setAlgorithmicDarkeningAllowed(webView.settings, false)
            }
            WebViewFeature.isFeatureSupported(WebViewFeature.FORCE_DARK) -> {
                WebSettingsCompat.setForceDark(webView.settings, WebSettingsCompat.FORCE_DARK_OFF)
            }
        }
        
        if (BuildConfig.DEBUG) {
            WebView.setWebContentsDebuggingEnabled(true)
        }
        
        webView.webChromeClient = object : WebChromeClient() {
            override fun onConsoleMessage(consoleMessage: ConsoleMessage): Boolean {
                Log.d(
                    "ImageStudioWebView",
                    "${consoleMessage.messageLevel()}: ${consoleMessage.message()} @ ${consoleMessage.sourceId()}:${consoleMessage.lineNumber()}",
                )
                return super.onConsoleMessage(consoleMessage)
            }
        }
        webView.webViewClient = object : WebViewClientCompat() {
            override fun shouldInterceptRequest(
                view: WebView,
                request: WebResourceRequest,
            ): WebResourceResponse? = assetLoader.shouldInterceptRequest(request.url)
        }
        webView.addJavascriptInterface(bridge, "AndroidImageStudio")
        val launchNonce = System.currentTimeMillis()
        webView.loadUrl("https://appassets.androidplatform.net/assets/index.html?target=${BuildConfig.TARGET_PLATFORM}&rev=$launchNonce")
    }

    override fun onConfigurationChanged(newConfig: Configuration) {
        super.onConfigurationChanged(newConfig)
        webView.post {
            webView.evaluateJavascript(
                """
                (() => {
                  window.dispatchEvent(new Event('resize'));
                  window.dispatchEvent(new Event('orientationchange'));
                  if (window.visualViewport) {
                    window.visualViewport.dispatchEvent(new Event('resize'));
                  }
                })();
                """.trimIndent(),
                null,
            )
        }
    }

    override fun onDestroy() {
        webView.removeJavascriptInterface("AndroidImageStudio")
        webView.destroy()
        super.onDestroy()
    }

    companion object {
        private const val notificationPermissionPrefs = "image_studio_android_permissions"
        private const val notificationPermissionRequestedKey = "background_task_notification_requested"
    }
}
