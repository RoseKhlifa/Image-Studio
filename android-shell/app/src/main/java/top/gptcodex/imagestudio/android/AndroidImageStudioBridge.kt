package top.gptcodex.imagestudio.android

import android.content.ContentValues
import android.content.Context
import android.content.Intent
import android.content.res.Configuration
import android.net.Uri
import android.os.Build
import android.os.Environment
import android.os.VibrationEffect
import android.os.Vibrator
import android.os.VibratorManager
import android.provider.MediaStore
import android.util.Base64
import android.webkit.JavascriptInterface
import android.webkit.WebView
import androidx.appcompat.app.AppCompatActivity
import androidx.core.content.FileProvider
import androidx.core.view.WindowCompat
import androidx.core.view.WindowInsetsCompat
import androidx.core.view.WindowInsetsControllerCompat
import org.json.JSONObject
import org.json.JSONArray
import android.provider.OpenableColumns
import java.io.File
import java.io.ByteArrayOutputStream
import java.io.FileInputStream
import java.io.FileOutputStream
import java.io.InputStream
import java.io.OutputStream
import java.net.HttpURLConnection
import java.net.InetSocketAddress
import java.net.URI
import java.net.Proxy
import java.net.URL
import java.text.SimpleDateFormat
import java.util.Date
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicReference
import java.util.Locale
import kotlin.concurrent.thread
import kotlin.math.max
import kotlin.math.roundToInt
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.WebSocket
import okhttp3.WebSocketListener
import okhttp3.Response
import java.security.SecureRandom
import java.security.cert.X509Certificate
import javax.net.ssl.HostnameVerifier
import javax.net.ssl.HttpsURLConnection
import javax.net.ssl.SSLContext
import javax.net.ssl.X509TrustManager

internal fun readLimitedBytes(input: InputStream, maxBytes: Int): ByteArray {
    require(maxBytes > 0) { "maxBytes must be positive" }
    val output = ByteArrayOutputStream(minOf(maxBytes, 64 * 1024))
    val buffer = ByteArray(16 * 1024)
    var total = 0
    while (true) {
        val read = input.read(buffer)
        if (read < 0) break
        total += read
        if (total > maxBytes) throw IllegalStateException("上游 URL 图片过大(>${maxBytes}B 上限)")
        output.write(buffer, 0, read)
    }
    return output.toByteArray()
}

internal fun boundedBinaryResponseLimit(requestedBytes: Long, hardMaxBytes: Long): Int {
    require(hardMaxBytes in 1..Int.MAX_VALUE.toLong()) { "hardMaxBytes is out of range" }
    val requested = if (requestedBytes > 0) requestedBytes else hardMaxBytes
    return minOf(requested, hardMaxBytes).toInt()
}

class AndroidImageStudioBridge(
    private val context: Context,
    private val webView: WebView,
    private val launchOpenImageDialog: () -> Unit,
    private val launchImportHistory: () -> Unit,
    private val ensureBackgroundTaskNotificationPermission: () -> Unit,
) {
    private val prefs = context.getSharedPreferences("image_studio_android", Context.MODE_PRIVATE)
    private val outputDirKey = "output_dir"
    private var pendingOpenImageRequestId: String? = null
    private var pendingImportHistoryRequestId: String? = null
    private val httpRequests = ConcurrentHashMap<String, HttpURLConnection>()
    private val webSocketRequests = ConcurrentHashMap<String, WebSocket>()
    @Volatile private var fullscreen = false
    // These are installed only after the current profile explicitly opts in.
    private val insecureTrustManager = object : X509TrustManager {
        override fun checkClientTrusted(chain: Array<out X509Certificate>?, authType: String?) = Unit
        override fun checkServerTrusted(chain: Array<out X509Certificate>?, authType: String?) = Unit
        override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()
    }
    private val insecureSSLContext by lazy {
        SSLContext.getInstance("TLS").apply {
            init(null, arrayOf(insecureTrustManager), SecureRandom())
        }
    }
    private val insecureHostnameVerifier = HostnameVerifier { _, _ -> true }

    companion object {
        private const val maxDialogReadBytes: Long = 50L * 1024L * 1024L
        private const val maxPreviewEdge = 384
        private const val generationConnectTimeoutMs = 30_000
        private const val generationReadTimeoutMs = 8 * 60 * 1000
    }

    @JavascriptInterface
    fun invoke(requestId: String, method: String, payloadJson: String) {
        try {
            val args = JSONArray(payloadJson)
            when (method) {
                "OpenImageDialog", "OpenMaskImageDialog" -> {
                    if (pendingOpenImageRequestId != null) {
                        throw IllegalStateException("图片选择已在进行中")
                    }
                    pendingOpenImageRequestId = requestId
                    launchOpenImageDialog()
                    return
                }
                "ImportHistoryFromFile" -> {
                    if (pendingImportHistoryRequestId != null) {
                        throw IllegalStateException("历史导入已在进行中")
                    }
                    pendingImportHistoryRequestId = requestId
                    launchImportHistory()
                    return
                }
            }
            val result: Any? = when (method) {
                "GetOutputDir" -> getOutputDir()
                "SetOutputDir" -> {
                    setOutputDir(args.optString(0, ""))
                    null
                }
                "ChooseOutputDir" -> getOutputDir()
                "GetStoredAPIKey" -> getStoredApiKey(args.optString(0))
                "SetStoredAPIKey" -> {
                    setStoredApiKey(args.optString(0), args.optString(1))
                    null
                }
                "DeleteStoredAPIKey" -> {
                    deleteStoredApiKey(args.optString(0))
                    null
                }
                "OpenExternalURL" -> {
                    openExternalUrl(args.optString(0))
                    null
                }
                "OpenOutputDir" -> {
                    openOutputDir()
                    null
                }
                "ImportImageFromB64" -> importImageFromB64(args.optString(0), args.optString(1))
                "ReadImageAsBase64" -> readImageAsBase64(args.optString(0))
                "ReadTextFile" -> readTextFile(args.optString(0))
                "OpenFile" -> {
                    openFile(args.optString(0))
                    null
                }
                "ExportHistoryToFile" -> exportHistory(args.optString(0))
                "SaveImageAs" -> saveImage(args.optString(0), args.optString(1))
                "SaveImagePathAs" -> saveImagePathAs(args.optString(0), args.optString(1))
                "SaveImageToDir" -> saveImageToDir(args.optString(0), args.optString(1), args.optString(2))
                "SaveImagePathToDir" -> saveImagePathToDir(args.optString(0), args.optString(1), args.optString(2))
                "HttpRequestText" -> {
                    val payload = args.optJSONObject(0) ?: throw IllegalArgumentException("缺少 HTTP 请求参数")
                    runHttpRequestText(requestId, payload)
                }
                "ResponsesWebSocketRequest" -> {
                    val payload = args.optJSONObject(0) ?: throw IllegalArgumentException("缺少 WebSocket 请求参数")
                    runResponsesWebSocketRequest(requestId, payload)
                }
                "ProbeUpstream" -> {
                    val payload = args.optJSONObject(0) ?: throw IllegalArgumentException("缺少测活参数")
                    runProbeUpstream(requestId, payload)
                }
                "CancelHttpRequest" -> {
                    cancelHttpRequest(args.optString(0))
                    null
                }
                "CancelWebSocketRequest" -> {
                    cancelWebSocketRequest(args.optString(0))
                    null
                }
                "Vibrate" -> {
                    vibrate(args.optLong(0, 50L))
                    null
                }
                "SetFullscreen" -> {
                    setFullscreen(args.optBoolean(0, false))
                    null
                }
                "IsFullscreen" -> fullscreen
                else -> throw UnsupportedOperationException("$method is not implemented in Android shell yet")
            }
            resolve(requestId, result)
        } catch (_: EarlyResolve) {
            return
        } catch (error: Exception) {
            reject(requestId, error.message ?: error.javaClass.simpleName)
        }
    }

    @JavascriptInterface
    fun getOutputDir(): String {
        return prefs.getString(outputDirKey, defaultOutputDir().absolutePath) ?: defaultOutputDir().absolutePath
    }

    @JavascriptInterface
    fun setOutputDir(path: String) {
        val dir = if (path.isBlank()) defaultOutputDir() else File(path)
        dir.mkdirs()
        prefs.edit().putString(outputDirKey, dir.absolutePath).apply()
    }

    @JavascriptInterface
    fun importImageFromB64(imageB64: String, suggestedName: String): Map<String, Any> {
        val bytes = Base64.decode(imageB64, Base64.DEFAULT)
        val file = writeImportedBytes(bytes, suggestedName)
        return mapOf(
            "path" to file.absolutePath,
            "imageB64" to imageB64,
        )
    }

    @JavascriptInterface
    fun getStoredApiKey(user: String): String {
        return prefs.getString("apikey_$user", "") ?: ""
    }

    @JavascriptInterface
    fun setStoredApiKey(user: String, value: String) {
        if (value.isBlank()) prefs.edit().remove("apikey_$user").apply()
        else prefs.edit().putString("apikey_$user", value.trim()).apply()
    }

    @JavascriptInterface
    fun deleteStoredApiKey(user: String) {
        prefs.edit().remove("apikey_$user").apply()
    }

    @JavascriptInterface
    fun getDisplayMetricsJson(): String {
        val dm = context.resources.displayMetrics
        val config = context.resources.configuration
        val widthPx = dm.widthPixels
        val heightPx = dm.heightPixels
        val orientation = when (config.orientation) {
            Configuration.ORIENTATION_LANDSCAPE -> "landscape"
            Configuration.ORIENTATION_PORTRAIT -> "portrait"
            else -> "undefined"
        }
        return JSONObject()
            .put("widthPx", widthPx)
            .put("heightPx", heightPx)
            .put("density", dm.density.toDouble())
            .put("densityDpi", dm.densityDpi)
            .put("screenWidthDp", config.screenWidthDp)
            .put("screenHeightDp", config.screenHeightDp)
            .put("smallestScreenWidthDp", config.smallestScreenWidthDp)
            .put("orientation", orientation)
            .toString()
    }

    @JavascriptInterface
    fun openOutputDir(): String {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
            val intent = Intent(Intent.ACTION_VIEW).apply {
                setDataAndType(MediaStore.Images.Media.EXTERNAL_CONTENT_URI, "image/*")
                addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
            }
            context.startActivity(intent)
            return "MediaStore"
        }
        val dir = File(getOutputDir()).apply { mkdirs() }
        val uri = FileProvider.getUriForFile(context, "${context.packageName}.fileprovider", dir)
        val intent = Intent(Intent.ACTION_VIEW).apply {
            setDataAndType(uri, "*/*")
            addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_GRANT_READ_URI_PERMISSION)
        }
        context.startActivity(intent)
        return dir.absolutePath
    }

    @JavascriptInterface
    fun openExternalUrl(url: String) {
        val intent = Intent(Intent.ACTION_VIEW, Uri.parse(url)).apply {
            addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
        }
        context.startActivity(intent)
    }

    @JavascriptInterface
    fun exportHistory(jsonContent: String): String {
        val file = File(getOutputDir(), "image-studio-history-${timestamp()}.json")
        file.parentFile?.mkdirs()
        file.writeText(jsonContent)
        return file.absolutePath
    }

    @JavascriptInterface
    fun readImageAsBase64(path: String): String {
        val bytes = openInputStreamForPath(path).use { it.readBytes() }
        return Base64.encodeToString(bytes, Base64.NO_WRAP)
    }

    @JavascriptInterface
    fun readTextFile(path: String): String {
        return openInputStreamForPath(path).bufferedReader().use { it.readText() }
    }

    @JavascriptInterface
    fun openFile(path: String) {
        val uriAndMime = uriAndMimeForPath(path)
        val intent = Intent(Intent.ACTION_VIEW).apply {
            setDataAndType(uriAndMime.first, uriAndMime.second)
            addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_GRANT_READ_URI_PERMISSION)
        }
        context.startActivity(intent)
    }

    @JavascriptInterface
    fun saveImage(imageB64: String, suggestedName: String): String {
        if (imageB64.isBlank()) throw IllegalArgumentException("图片数据为空")
        val name = ensureImageFileName(suggestedName, "image-${timestamp()}.png")
        val bytes = Base64.decode(imageB64, Base64.DEFAULT)
        return saveImageStream(name, imageMimeForName(name)) { output ->
            output.write(bytes)
        }
    }

    @JavascriptInterface
    fun saveImagePathAs(path: String, suggestedName: String): String {
        val trimmed = path.trim()
        if (trimmed.isBlank()) throw IllegalArgumentException("图片路径为空")
        val fallback = trimmed.substringAfterLast('/').substringAfterLast('\\').ifBlank { "image-${timestamp()}.png" }
        val name = ensureImageFileName(suggestedName.ifBlank { fallback }, fallback)
        return saveImageStream(name, imageMimeForName(name)) { output ->
            openInputStreamForPath(trimmed).use { input ->
                input.copyTo(output)
            }
        }
    }

    @JavascriptInterface
    fun saveImageToDir(imageB64: String, directory: String, suggestedName: String): String {
        if (imageB64.isBlank()) throw IllegalArgumentException("图片数据为空")
        val targetDir = ensureWritableDirectory(directory)
        val name = ensureImageFileName(suggestedName, "image-${timestamp()}.png")
        val bytes = Base64.decode(imageB64, Base64.DEFAULT)
        val file = uniqueTargetFile(targetDir, name)
        FileOutputStream(file).use { output -> output.write(bytes) }
        return file.absolutePath
    }

    @JavascriptInterface
    fun saveImagePathToDir(path: String, directory: String, suggestedName: String): String {
        val trimmed = path.trim()
        if (trimmed.isBlank()) throw IllegalArgumentException("图片路径为空")
        val targetDir = ensureWritableDirectory(directory)
        val fallback = trimmed.substringAfterLast('/').substringAfterLast('\\').ifBlank { "image-${timestamp()}.png" }
        val name = ensureImageFileName(suggestedName.ifBlank { fallback }, fallback)
        val file = uniqueTargetFile(targetDir, name)
        openInputStreamForPath(trimmed).use { input ->
            FileOutputStream(file).use { output -> input.copyTo(output) }
        }
        return file.absolutePath
    }

    private fun saveImageStream(name: String, mimeType: String, write: (OutputStream) -> Unit): String {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
            val contentValues = ContentValues().apply {
                put(MediaStore.MediaColumns.DISPLAY_NAME, name)
                put(MediaStore.MediaColumns.MIME_TYPE, mimeType)
                put(MediaStore.MediaColumns.RELATIVE_PATH, Environment.DIRECTORY_PICTURES + File.separator + "ImageStudio")
                put(MediaStore.MediaColumns.IS_PENDING, 1)
            }

            val resolver = context.contentResolver
            val uri = resolver.insert(MediaStore.Images.Media.EXTERNAL_CONTENT_URI, contentValues)
            if (uri != null) {
                try {
                    resolver.openOutputStream(uri)?.use { output ->
                        write(output)
                    } ?: throw IllegalStateException("无法写入相册文件")
                } catch (error: Exception) {
                    resolver.delete(uri, null, null)
                    throw error
                }
                contentValues.clear()
                contentValues.put(MediaStore.MediaColumns.IS_PENDING, 0)
                resolver.update(uri, contentValues, null, null)
                return uri.toString()
            }
        }

        val file = File(getOutputDir(), name)
        file.parentFile?.mkdirs()
        FileOutputStream(file).use { output ->
            write(output)
        }
        return file.absolutePath
    }

    private fun ensureWritableDirectory(path: String): File {
        val trimmed = path.trim()
        if (trimmed.isBlank()) throw IllegalArgumentException("目标目录为空")
        val dir = File(trimmed)
        dir.mkdirs()
        return dir
    }

    private fun uniqueTargetFile(directory: File, suggestedName: String): File {
        val baseName = ensureImageFileName(suggestedName, "image-${timestamp()}.png")
        var candidate = File(directory, baseName)
        if (!candidate.exists()) return candidate
        val dot = baseName.lastIndexOf('.')
        val stem = if (dot >= 0) baseName.substring(0, dot) else baseName
        val ext = if (dot >= 0) baseName.substring(dot) else ""
        var index = 2
        while (candidate.exists()) {
            candidate = File(directory, "$stem-$index$ext")
            index += 1
        }
        return candidate
    }

    @JavascriptInterface
    fun vibrate(milliseconds: Long) {
        val vibrator = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
            val vibratorManager = context.getSystemService(Context.VIBRATOR_MANAGER_SERVICE) as VibratorManager
            vibratorManager.defaultVibrator
        } else {
            @Suppress("DEPRECATION")
            context.getSystemService(Context.VIBRATOR_SERVICE) as Vibrator
        }
        
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            vibrator.vibrate(VibrationEffect.createOneShot(milliseconds, VibrationEffect.DEFAULT_AMPLITUDE))
        } else {
            @Suppress("DEPRECATION")
            vibrator.vibrate(milliseconds)
        }
    }

    @JavascriptInterface
    fun setFullscreen(enabled: Boolean) {
        fullscreen = enabled
        val activity = context as? AppCompatActivity ?: return
        activity.runOnUiThread {
            WindowCompat.setDecorFitsSystemWindows(activity.window, false)
            val controller = WindowInsetsControllerCompat(activity.window, webView)
            if (enabled) {
                controller.systemBarsBehavior = WindowInsetsControllerCompat.BEHAVIOR_SHOW_TRANSIENT_BARS_BY_SWIPE
                controller.hide(WindowInsetsCompat.Type.systemBars())
            } else {
                controller.show(WindowInsetsCompat.Type.systemBars())
            }
            webView.post {
                webView.evaluateJavascript(
                    """
                    (() => {
                      window.dispatchEvent(new Event('resize'));
                      if (window.visualViewport) {
                        window.visualViewport.dispatchEvent(new Event('resize'));
                      }
                    })();
                    """.trimIndent(),
                    null,
                )
            }
        }
    }

    private fun runHttpRequestText(requestId: String, payload: JSONObject): Nothing {
        val requestKey = payload.optString("requestKey").ifBlank { requestId }
        val url = payload.optString("url").trim()
        val method = payload.optString("method", "GET").trim().uppercase(Locale.US)
        val headersJson = payload.optJSONObject("headers")
        val bodyBase64 = payload.optString("bodyBase64")
        val contentType = payload.optString("contentType")
        val streamLines = payload.optBoolean("streamLines", false)
        val proxyMode = payload.optString("proxyMode", "system")
        val proxyUrl = payload.optString("proxyURL", "")
        val allowInsecureConnection = payload.optBoolean("allowInsecureConnection", false)
        val keepAlive = payload.optBoolean("keepAlive", false)
        val responseBase64 = payload.optBoolean("responseBase64", false)
        val requestedMaxResponseBytes = payload.optLong("maxResponseBytes", 0L)
        val maxResponseBytes = boundedBinaryResponseLimit(requestedMaxResponseBytes, maxDialogReadBytes)
        if (keepAlive) {
            ensureBackgroundTaskNotificationPermission()
            GenerationForegroundService.acquire(context, requestKey)
        }
        thread(name = "image-studio-http-$requestKey") {
            try {
                val connection = openHttpConnection(url, proxyMode, proxyUrl, allowInsecureConnection).apply {
                    requestMethod = method
                    instanceFollowRedirects = true
                    connectTimeout = generationConnectTimeoutMs
                    // Multi-image / high-resolution jobs can legitimately run
                    // for minutes after the last preview frame. Keeping the
                    // Android native HTTP bridge at only 180s makes it more
                    // likely to cut the stream before the final image arrives.
                    readTimeout = generationReadTimeoutMs
                    doInput = true
                }
                httpRequests[requestKey] = connection
                if (headersJson != null) {
                    val keys = headersJson.keys()
                    while (keys.hasNext()) {
                        val key = keys.next()
                        connection.setRequestProperty(key, headersJson.optString(key))
                    }
                }
                if (contentType.isNotBlank() && connection.getRequestProperty("Content-Type").isNullOrBlank()) {
                    connection.setRequestProperty("Content-Type", contentType)
                }
                val requestBytes = if (bodyBase64.isBlank()) ByteArray(0) else Base64.decode(bodyBase64, Base64.DEFAULT)
                if (requestBytes.isNotEmpty()) {
                    connection.doOutput = true
                    connection.outputStream.use { it.write(requestBytes) }
                }
                val status = connection.responseCode
                val stream = if (status >= 400) connection.errorStream else connection.inputStream
                var streamResult: NativeHttpStreamResultSnapshot? = null
                var binaryResponseB64 = ""
                val body = if (responseBase64 && status in 200..299) {
                    val contentLength = connection.contentLengthLong
                    if (contentLength > maxResponseBytes) {
                        throw IllegalStateException("上游 URL 图片过大(${contentLength}B > ${maxResponseBytes}B 上限)")
                    }
                    val bytes = stream?.use { readLimitedBytes(it, maxResponseBytes) } ?: ByteArray(0)
                    binaryResponseB64 = Base64.encodeToString(bytes, Base64.NO_WRAP)
                    ""
                } else if (streamLines) {
                    val bodyBuilder = StringBuilder()
                    stream?.bufferedReader()?.useLines { sequence ->
                        sequence.forEach { line ->
                            bodyBuilder.append(line).append('\n')
                            if (streamResult == null) {
                                streamResult = extractNativeHttpStreamResult(line)
                            }
                            buildNativeHttpStreamProgressPayload(line)?.let { progressPayload ->
                                emitNativeProgress(requestKey, progressPayload)
                            }
                        }
                    }
                    bodyBuilder.toString()
                } else {
                    stream?.bufferedReader()?.use { it.readText() } ?: ""
                }
                val rawPath = if (body.isNotBlank()) writeNativeHttpRawBody(body, connection.contentType ?: "", requestKey) else ""
                val result = mapOf(
                    "status" to status,
                    "body" to if (streamResult != null) "" else body,
                    "contentType" to (connection.contentType ?: ""),
                    "rawPath" to rawPath,
                    "resultImageB64" to binaryResponseB64.ifBlank { streamResult?.imageB64 ?: "" },
                    "revisedPrompt" to (streamResult?.revisedPrompt ?: ""),
                    "sourceEvent" to if (binaryResponseB64.isNotBlank()) "url_download" else (streamResult?.sourceEvent ?: ""),
                )
                resolve(requestId, result)
            } catch (error: Exception) {
                reject(requestId, error.message ?: error.javaClass.simpleName)
            } finally {
                try {
                    httpRequests.remove(requestKey)?.disconnect()
                } finally {
                    if (keepAlive) GenerationForegroundService.release(context, requestKey)
                }
            }
        }
        throw EarlyResolve()
    }

    private fun writeNativeHttpRawBody(body: String, contentType: String, requestKey: String): String {
        val logDir = File(getOutputDir(), "log").apply { mkdirs() }
        val ext = if (contentType.lowercase(Locale.US).contains("json")) "json" else "txt"
        val file = File(logDir, "android-http-${timestamp()}-${sanitizeFileName(requestKey, "request")}.$ext")
        file.writeText(body)
        return file.absolutePath
    }

    private fun runProbeUpstream(requestId: String, payload: JSONObject): Nothing {
        val allowInsecureConnection = payload.optBoolean("allowInsecureConnection", false)
        val baseUrl = validateProbeBaseUrl(payload.optString("baseURL"), allowInsecureConnection)
        val apiKey = payload.optString("apiKey").trim()
        val proxyMode = payload.optString("proxyMode", "system")
        val proxyUrl = payload.optString("proxyURL", "")
        val provider = payload.optString("provider", "openai").lowercase(Locale.US).let {
            if (it == "google" || it == "grok") it else "openai"
        }
        val apiMode = payload.optString("apiMode", "responses")
        val responsesTransport = payload.optString("responsesTransport", "sse")
        if (apiKey.isBlank()) throw IllegalArgumentException("API Key 不能为空")
        thread(name = "image-studio-probe-${requestId.take(12)}") {
            try {
                val endpointLabel = if (provider == "google") "/v1beta/models" else "/v1/models"
                val modelsEndpoint = if (provider == "google") googleEndpoint(baseUrl, "models") else openAIEndpoint(baseUrl, "models")
                val connection = openHttpConnection(modelsEndpoint, proxyMode, proxyUrl, allowInsecureConnection).apply {
                    requestMethod = "GET"
                    instanceFollowRedirects = true
                    connectTimeout = 20_000
                    readTimeout = 20_000
                    doInput = true
                    if (provider == "google") setRequestProperty("X-Goog-Api-Key", apiKey)
                    else setRequestProperty("Authorization", "Bearer $apiKey")
                    setRequestProperty("Accept", "application/json")
                    setRequestProperty("User-Agent", "image-studio-android")
                }
                val status = connection.responseCode
                val stream = if (status >= 400) connection.errorStream else connection.inputStream
                val body = stream?.bufferedReader()?.use { reader ->
                    reader.readText().take(1_048_576)
                } ?: ""
                connection.disconnect()
                if (status !in 200..299) {
                    throw IllegalStateException("上游 $endpointLabel 返回 $status${summarizeProbeBody(body).let { if (it.isBlank()) "" else ": $it" }}")
                }
                val parsed = JSONObject(body)
                val data = if (provider == "google") parsed.optJSONArray("models")
                    ?: throw IllegalStateException("上游 /v1beta/models 响应缺少 models 数组")
                else parsed.optJSONArray("data")
                    ?: throw IllegalStateException("上游 /v1/models 响应缺少 data 数组")
                val models = mutableListOf<Map<String, String>>()
                for (index in 0 until data.length()) {
                    val item = data.optJSONObject(index) ?: continue
                    val id = if (provider == "google") item.optString("name").trim().removePrefix("models/") else item.optString("id").trim()
                    if (id.isBlank()) continue
                    models += mapOf(
                        "id" to id,
                        "object" to if (provider == "google") "model" else item.optString("object").trim(),
                        "ownedBy" to if (provider == "google") "google" else item.optString("owned_by").trim(),
                        "displayName" to if (provider == "google") item.optString("displayName").trim() else item.optString("name").trim(),
                    )
                }
                val result = mutableMapOf<String, Any>(
                    "modelCount" to data.length(),
                    "models" to models,
                )
                if (provider == "openai" && apiMode == "responses" && responsesTransport == "websocket") {
                    result["responsesTransport"] = "websocket"
                    val probePayload = JSONObject()
                        .put("type", "response.create")
                        .put("model", "gpt-5.5")
                        .put("store", false)
                        .put("generate", false)
                        .put("input", JSONArray().put(
                            JSONObject()
                                .put("role", "user")
                                .put("content", JSONArray().put(
                                    JSONObject()
                                        .put("type", "input_text")
                                        .put("text", "health check")
                                ))
                        ))
                        .put("tools", JSONArray())
                    try {
                        probeResponsesWebSocket(baseUrl, apiKey, probePayload.toString(), proxyMode, proxyUrl, allowInsecureConnection)
                        result["responsesTransportOK"] = true
                    } catch (wsError: Exception) {
                        result["responsesTransportOK"] = false
                        result["responsesTransportError"] = wsError.message ?: wsError.javaClass.simpleName
                    }
                }
                resolve(requestId, result)
            } catch (error: Exception) {
                reject(requestId, error.message ?: error.javaClass.simpleName)
            }
        }
        throw EarlyResolve()
    }

    private fun openHttpConnection(
        url: String,
        proxyMode: String,
        proxyUrl: String,
        allowInsecureConnection: Boolean = false,
    ): HttpURLConnection {
        val target = URL(validateRequestUrl(url, allowInsecureConnection))
        val connection = when (normalizeProxyMode(proxyMode)) {
            "none" -> target.openConnection(Proxy.NO_PROXY)
            "custom" -> target.openConnection(parseCustomProxy(proxyUrl))
            else -> target.openConnection()
        }
        val httpConnection = connection as HttpURLConnection
        if (allowInsecureConnection && httpConnection is HttpsURLConnection) {
            httpConnection.sslSocketFactory = insecureSSLContext.socketFactory
            httpConnection.hostnameVerifier = insecureHostnameVerifier
        }
        return httpConnection
    }

    private fun normalizeProxyMode(raw: String): String {
        return when (raw.trim().lowercase(Locale.US)) {
            "none" -> "none"
            "custom" -> "custom"
            else -> "system"
        }
    }

    private fun parseCustomProxy(raw: String): Proxy {
        val cleaned = raw.trim()
        if (cleaned.isBlank()) throw IllegalArgumentException("自定义代理地址不能为空")
        val uri = try {
            URI(cleaned)
        } catch (error: Exception) {
            throw IllegalArgumentException("代理地址无效: ${error.message ?: error.javaClass.simpleName}")
        }
        val scheme = uri.scheme?.lowercase(Locale.US) ?: ""
        if (scheme != "http" && scheme != "https") {
            throw IllegalArgumentException("代理地址仅支持 http:// 或 https://")
        }
        val host = uri.host ?: throw IllegalArgumentException("代理地址必须包含主机")
        if (!uri.rawQuery.isNullOrBlank() || !uri.rawFragment.isNullOrBlank()) {
            throw IllegalArgumentException("代理地址不能包含 query 或 fragment")
        }
        val path = uri.rawPath ?: ""
        if (path.isNotBlank() && path != "/") {
            throw IllegalArgumentException("代理地址不能包含路径")
        }
        val port = if (uri.port > 0) uri.port else if (scheme == "https") 443 else 80
        return Proxy(Proxy.Type.HTTP, InetSocketAddress.createUnresolved(host, port))
    }

    private fun cancelHttpRequest(requestKey: String) {
        httpRequests.remove(requestKey)?.disconnect()
    }

    private fun cancelWebSocketRequest(requestKey: String) {
        webSocketRequests.remove(requestKey)?.cancel()
    }

    private fun runResponsesWebSocketRequest(requestId: String, payload: JSONObject): Nothing {
        val requestKey = payload.optString("requestKey").ifBlank { requestId }
        val allowInsecureConnection = payload.optBoolean("allowInsecureConnection", false)
        val baseUrl = validateProbeBaseUrl(payload.optString("baseURL"), allowInsecureConnection)
        val apiKey = payload.optString("apiKey").trim()
        val proxyMode = payload.optString("proxyMode", "system")
        val proxyUrl = payload.optString("proxyURL", "")
        val payloadText = payload.optString("payload")
        if (apiKey.isBlank()) throw IllegalArgumentException("API Key 不能为空")
        if (payloadText.isBlank()) throw IllegalArgumentException("WebSocket payload 不能为空")
        val wsUrl = responsesWebSocketUrl(baseUrl)
        val client = okHttpClientForWebSocket(proxyMode, proxyUrl, allowInsecureConnection)
        val rawLines = mutableListOf<String>()
        val streamResult = arrayOfNulls<NativeHttpStreamResultSnapshot>(1)
        val settled = java.util.concurrent.atomic.AtomicBoolean(false)
        val keepAliveReleased = java.util.concurrent.atomic.AtomicBoolean(false)
        val releaseKeepAlive = {
            if (keepAliveReleased.compareAndSet(false, true)) {
                GenerationForegroundService.release(context, requestKey)
            }
        }
        ensureBackgroundTaskNotificationPermission()
        GenerationForegroundService.acquire(context, requestKey)
        val socket = try {
            client.newWebSocket(
                Request.Builder()
                    .url(wsUrl)
                    .addHeader("Authorization", "Bearer $apiKey")
                    .addHeader("User-Agent", "image-studio-android")
                    .build(),
                object : WebSocketListener() {
                override fun onOpen(webSocket: WebSocket, response: Response) {
                    webSocketRequests[requestKey] = webSocket
                    webSocket.send(payloadText)
                }

                override fun onMessage(webSocket: WebSocket, text: String) {
                    val line = text.trim()
                    if (line.isBlank()) return
                    rawLines += "data: $line"
                    if (streamResult[0] == null) {
                        streamResult[0] = extractNativeHttpStreamResult(line)
                    }
                    buildNativeHttpStreamProgressPayload(line)?.let { progressPayload ->
                        emitNativeProgress(requestKey, progressPayload)
                    }
                    try {
                        val parsed = JSONObject(line)
                        when (parsed.optString("type")) {
                            "response.completed" -> {
                                if (settled.compareAndSet(false, true)) {
                                    val rawBody = rawLines.joinToString("\n")
                                    val rawPath = if (rawBody.isNotBlank()) writeNativeHttpRawBody(rawBody, "text/plain", requestKey) else ""
                                    resolve(requestId, mapOf(
                                        "status" to 200,
                                        "body" to "",
                                        "contentType" to "text/event-stream",
                                        "rawPath" to rawPath,
                                        "resultImageB64" to (streamResult[0]?.imageB64 ?: ""),
                                        "revisedPrompt" to (streamResult[0]?.revisedPrompt ?: ""),
                                        "sourceEvent" to (streamResult[0]?.sourceEvent ?: "final"),
                                    ))
                                    webSocket.close(1000, "completed")
                                    webSocketRequests.remove(requestKey)
                                    releaseKeepAlive()
                                }
                            }
                            "error" -> {
                                if (settled.compareAndSet(false, true)) {
                                    reject(requestId, line)
                                    webSocket.cancel()
                                    webSocketRequests.remove(requestKey)
                                    releaseKeepAlive()
                                }
                            }
                        }
                    } catch (_: Exception) {
                    }
                }

                override fun onFailure(webSocket: WebSocket, t: Throwable, response: Response?) {
                    webSocketRequests.remove(requestKey)
                    releaseKeepAlive()
                    if (settled.compareAndSet(false, true)) {
                        reject(requestId, t.message ?: "websocket failure")
                    }
                }

                override fun onClosed(webSocket: WebSocket, code: Int, reason: String) {
                    webSocketRequests.remove(requestKey)
                    releaseKeepAlive()
                    if (settled.compareAndSet(false, true)) {
                        if (streamResult[0]?.imageB64?.isNotBlank() == true) {
                            val rawBody = rawLines.joinToString("\n")
                            val rawPath = if (rawBody.isNotBlank()) writeNativeHttpRawBody(rawBody, "text/plain", requestKey) else ""
                            resolve(requestId, mapOf(
                                "status" to 200,
                                "body" to "",
                                "contentType" to "text/event-stream",
                                "rawPath" to rawPath,
                                "resultImageB64" to (streamResult[0]?.imageB64 ?: ""),
                                "revisedPrompt" to (streamResult[0]?.revisedPrompt ?: ""),
                                "sourceEvent" to (streamResult[0]?.sourceEvent ?: "final"),
                            ))
                        } else {
                            reject(requestId, if (reason.isNotBlank()) reason else "websocket closed before final image")
                        }
                    }
                }
                },
            )
        } catch (error: Exception) {
            releaseKeepAlive()
            throw error
        }
        webSocketRequests[requestKey] = socket
        throw EarlyResolve()
    }

    private fun responsesWebSocketUrl(baseUrl: String): String {
        val uri = URI(baseUrl)
        val scheme = when (uri.scheme?.lowercase(Locale.US)) {
            "https" -> "wss"
            "http" -> "ws"
            else -> throw IllegalArgumentException("BASE_URL 仅支持 http:// 或 https://")
        }
        val path = (uri.path ?: "").trimEnd('/')
        val normalizedPath = if (path.isBlank()) "/v1/responses" else "$path/v1/responses"
        return URI(scheme, uri.userInfo, uri.host, uri.port, normalizedPath, null, null).toString()
    }

    private fun okHttpClientForWebSocket(
        proxyMode: String,
        proxyUrl: String,
        allowInsecureConnection: Boolean = false,
    ): OkHttpClient {
        val builder = OkHttpClient.Builder()
            .connectTimeout(generationConnectTimeoutMs.toLong(), java.util.concurrent.TimeUnit.MILLISECONDS)
            .readTimeout(0, java.util.concurrent.TimeUnit.MILLISECONDS)
            .pingInterval(25, java.util.concurrent.TimeUnit.SECONDS)
        when (normalizeProxyMode(proxyMode)) {
            "none" -> builder.proxy(Proxy.NO_PROXY)
            "custom" -> builder.proxy(parseCustomProxy(proxyUrl))
        }
        if (allowInsecureConnection) {
            builder.sslSocketFactory(insecureSSLContext.socketFactory, insecureTrustManager)
            builder.hostnameVerifier(insecureHostnameVerifier)
        }
        return builder.build()
    }

    private fun probeResponsesWebSocket(
        baseUrl: String,
        apiKey: String,
        payload: String,
        proxyMode: String,
        proxyUrl: String,
        allowInsecureConnection: Boolean,
    ) {
        val wsUrl = responsesWebSocketUrl(baseUrl)
        val client = okHttpClientForWebSocket(proxyMode, proxyUrl, allowInsecureConnection)
        val done = CountDownLatch(1)
        val errorRef = AtomicReference<String?>(null)
        val socket = client.newWebSocket(
            Request.Builder()
                .url(wsUrl)
                .addHeader("Authorization", "Bearer $apiKey")
                .addHeader("User-Agent", "image-studio-android")
                .build(),
            object : WebSocketListener() {
                override fun onOpen(webSocket: WebSocket, response: Response) {
                    webSocket.send(payload)
                }

                override fun onMessage(webSocket: WebSocket, text: String) {
                    try {
                        val parsed = JSONObject(text)
                        when (parsed.optString("type")) {
                            "response.created", "response.completed" -> {
                                webSocket.close(1000, "probe completed")
                                done.countDown()
                            }
                            "error" -> {
                                errorRef.set(text)
                                webSocket.cancel()
                                done.countDown()
                            }
                        }
                    } catch (_: Exception) {
                    }
                }

                override fun onFailure(webSocket: WebSocket, t: Throwable, response: Response?) {
                    errorRef.set(t.message ?: "websocket failure")
                    done.countDown()
                }

                override fun onClosed(webSocket: WebSocket, code: Int, reason: String) {
                    done.countDown()
                }
            }
        )
        if (!done.await(20, TimeUnit.SECONDS)) {
            socket.cancel()
            throw IllegalStateException("Responses WebSocket 探测超时")
        }
        errorRef.get()?.let { throw IllegalStateException(it) }
    }

    private fun validateProbeBaseUrl(raw: String, allowInsecureConnection: Boolean = false): String {
        val trimmed = raw.trim().trimEnd('/')
        val cleaned = if (trimmed.endsWith("/v1", ignoreCase = true)) trimmed.dropLast(3).trimEnd('/') else trimmed
        if (cleaned.isBlank()) throw IllegalArgumentException("未配置上游 BASE_URL")
        val uri = try {
            URI(cleaned)
        } catch (error: Exception) {
            throw IllegalArgumentException("BASE_URL 无效: ${error.message ?: error.javaClass.simpleName}")
        }
        val scheme = uri.scheme?.lowercase(Locale.US) ?: ""
        val host = uri.host ?: ""
        if (scheme.isBlank() || host.isBlank()) {
            throw IllegalArgumentException("BASE_URL 必须包含协议和主机,例如 https://example.com")
        }
        if (scheme == "https") return cleaned
        if (scheme == "http" && (allowInsecureConnection || isProbeLoopbackHost(host))) return cleaned
        if (scheme == "http") {
            throw IllegalArgumentException("拒绝使用非 TLS 上游: $cleaned。只有 localhost / 127.0.0.1 / ::1 允许 http://")
        }
        throw IllegalArgumentException("BASE_URL 仅支持 http:// 或 https://")
    }

    private fun validateRequestUrl(raw: String, allowInsecureConnection: Boolean): String {
        val cleaned = raw.trim()
        if (cleaned.isBlank()) throw IllegalArgumentException("请求 URL 不能为空")
        val uri = try {
            URI(cleaned)
        } catch (error: Exception) {
            throw IllegalArgumentException("请求 URL 无效: ${error.message ?: error.javaClass.simpleName}")
        }
        val scheme = uri.scheme?.lowercase(Locale.US) ?: ""
        val host = uri.host ?: ""
        if (scheme.isBlank() || host.isBlank()) {
            throw IllegalArgumentException("请求 URL 必须包含协议和主机")
        }
        if (scheme == "https") return cleaned
        if (scheme == "http" && (allowInsecureConnection || isProbeLoopbackHost(host))) return cleaned
        if (scheme == "http") {
            throw IllegalArgumentException("拒绝使用非 TLS 上游: $cleaned。请在渠道中开启“允许不安全连接”")
        }
        throw IllegalArgumentException("请求 URL 仅支持 http:// 或 https://")
    }

    private fun openAIEndpoint(baseUrl: String, endpointPath: String): String {
        val cleaned = baseUrl.trim().trimEnd('/')
        val path = endpointPath.trim().trim('/')
        if (path.isBlank()) return cleaned
        val uri = URI(cleaned)
        val basePath = (uri.path ?: "").trimEnd('/').lowercase(Locale.US)
        return if (basePath.endsWith("/openai")) "$cleaned/$path" else "$cleaned/v1/$path"
    }

    private fun googleEndpoint(baseUrl: String, endpointPath: String): String {
        var cleaned = baseUrl.trim().trimEnd('/')
        val path = endpointPath.trim().trim('/')
        cleaned = when {
            cleaned.endsWith("/v1beta/openai", ignoreCase = true) -> cleaned.dropLast("/openai".length)
            cleaned.endsWith("/v1beta", ignoreCase = true) -> cleaned
            else -> "$cleaned/v1beta"
        }
        return if (path.isBlank()) cleaned else "$cleaned/$path"
    }

    private fun isProbeLoopbackHost(host: String): Boolean {
        val lower = host.lowercase(Locale.US).trim('[', ']')
        return lower == "localhost" || lower.endsWith(".localhost") || lower == "127.0.0.1" || lower == "::1" || lower == "0:0:0:0:0:0:0:1"
    }

    private fun summarizeProbeBody(body: String): String {
        val trimmed = body.trim()
        if (trimmed.isBlank()) return ""
        return try {
            val parsed = JSONObject(trimmed)
            val message = parsed.optJSONObject("error")?.optString("message")?.trim().orEmpty()
            val fallback = parsed.optString("message").trim()
            (message.ifBlank { fallback }).ifBlank { trimmed }.take(160)
        } catch (_: Exception) {
            trimmed.take(160)
        }
    }

    fun onOpenImageDialogResult(uri: Uri?) {
        val requestId = pendingOpenImageRequestId ?: return
        pendingOpenImageRequestId = null
        if (uri == null) {
            resolve(requestId, mapOf("path" to "", "size" to 0, "imageB64" to ""))
            return
        }
        try {
            val suggestedName = queryDisplayName(uri) ?: "import-${timestamp()}.png"
            val copied = copyUriToImports(uri, suggestedName)
            resolve(
                requestId,
                mapOf(
                    "path" to copied.file.absolutePath,
                    "size" to copied.size,
                    "imageB64" to copied.imageB64,
                    "previewB64" to copied.previewB64,
                ),
            )
        } catch (error: Exception) {
            reject(requestId, error.message ?: error.javaClass.simpleName)
        }
    }

    fun onImportHistoryResult(uri: Uri?) {
        val requestId = pendingImportHistoryRequestId ?: return
        pendingImportHistoryRequestId = null
        if (uri == null) {
            resolve(requestId, "")
            return
        }
        try {
            val text = context.contentResolver.openInputStream(uri)?.bufferedReader()?.use { it.readText() } ?: ""
            resolve(requestId, text)
        } catch (error: Exception) {
            reject(requestId, error.message ?: error.javaClass.simpleName)
        }
    }

    private fun defaultOutputDir(): File {
        val pictures = context.getExternalFilesDir(Environment.DIRECTORY_PICTURES)
        return File(pictures ?: context.filesDir, "ImageStudio")
    }

    private fun importsDir(): File {
        return File(context.filesDir, "imports").apply { mkdirs() }
    }

    private fun sanitizeFileName(name: String, fallback: String): String {
        val trimmed = name.trim()
        if (trimmed.isEmpty()) return fallback
        return trimmed.replace(Regex("[^A-Za-z0-9._\\-\\u4E00-\\u9FFF]+"), "-")
    }

    private fun ensureImageFileName(name: String, fallback: String): String {
        val safe = sanitizeFileName(name, fallback)
        return if (Regex("\\.(png|jpe?g|webp)$", RegexOption.IGNORE_CASE).containsMatchIn(safe)) {
            safe
        } else {
            "$safe.png"
        }
    }

    private fun imageMimeForName(name: String): String {
        val lower = name.lowercase(Locale.US)
        return when {
            lower.endsWith(".jpg") || lower.endsWith(".jpeg") -> "image/jpeg"
            lower.endsWith(".webp") -> "image/webp"
            else -> "image/png"
        }
    }

    private fun queryDisplayName(uri: Uri): String? {
        context.contentResolver.query(uri, arrayOf(OpenableColumns.DISPLAY_NAME), null, null, null)?.use { cursor ->
            val index = cursor.getColumnIndex(OpenableColumns.DISPLAY_NAME)
            if (index >= 0 && cursor.moveToFirst()) {
                val name = cursor.getString(index)
                if (!name.isNullOrBlank()) return name
            }
        }
        return null
    }

    private data class CopiedImport(
        val file: File,
        val size: Long,
        val imageB64: String,
        val previewB64: String,
    )

    private fun copyUriToImports(uri: Uri, suggestedName: String): CopiedImport {
        val name = sanitizeFileName(suggestedName, "import-${timestamp()}.png")
        val target = File(importsDir(), "${timestamp()}-$name")
        var total = 0L
        val preview = java.io.ByteArrayOutputStream()
        context.contentResolver.openInputStream(uri)?.use { input ->
            FileOutputStream(target).use { output ->
                val buffer = ByteArray(8192)
                while (true) {
                    val read = input.read(buffer)
                    if (read <= 0) break
                    output.write(buffer, 0, read)
                    total += read
                    if (total <= maxDialogReadBytes) {
                        preview.write(buffer, 0, read)
                    }
                }
            }
        } ?: throw IllegalStateException("无法读取所选文件")
        val imageB64 = if (total in 1..maxDialogReadBytes) {
            Base64.encodeToString(preview.toByteArray(), Base64.NO_WRAP)
        } else {
            ""
        }
        return CopiedImport(target, total, imageB64, createPreviewB64(target))
    }

    private fun createPreviewB64(file: File): String {
        val bounds = BitmapFactory.Options().apply { inJustDecodeBounds = true }
        BitmapFactory.decodeFile(file.absolutePath, bounds)
        if (bounds.outWidth <= 0 || bounds.outHeight <= 0) return ""
        val maxEdge = max(bounds.outWidth, bounds.outHeight)
        var sample = 1
        while (maxEdge / sample > maxPreviewEdge * 2) sample *= 2
        val decode = BitmapFactory.Options().apply { inSampleSize = sample }
        val decoded = BitmapFactory.decodeFile(file.absolutePath, decode) ?: return ""
        val scaled = try {
            val scale = minOf(1f, maxPreviewEdge.toFloat() / max(decoded.width, decoded.height).toFloat())
            if (scale >= 0.999f) decoded
            else Bitmap.createScaledBitmap(
                decoded,
                max(1, (decoded.width * scale).roundToInt()),
                max(1, (decoded.height * scale).roundToInt()),
                true,
            )
        } catch (_: Exception) {
            decoded
        }
        return try {
            val out = java.io.ByteArrayOutputStream()
            scaled.compress(Bitmap.CompressFormat.JPEG, 74, out)
            Base64.encodeToString(out.toByteArray(), Base64.NO_WRAP)
        } finally {
            if (scaled != decoded) scaled.recycle()
            decoded.recycle()
        }
    }

    private fun writeImportedBytes(bytes: ByteArray, suggestedName: String): File {
        val safeName = sanitizeFileName(suggestedName, "import-${timestamp()}.png")
        val file = File(importsDir(), "${timestamp()}-$safeName")
        file.writeBytes(bytes)
        return file
    }

    private fun openInputStreamForPath(path: String): InputStream {
        val trimmed = path.trim()
        if (trimmed.startsWith("content://")) {
            return context.contentResolver.openInputStream(Uri.parse(trimmed))
                ?: throw IllegalArgumentException("无法读取内容 URI: $trimmed")
        }
        return FileInputStream(File(trimmed))
    }

    private fun uriAndMimeForPath(path: String): Pair<Uri, String> {
        val trimmed = path.trim()
        val mime = when {
            trimmed.endsWith(".png", true) -> "image/png"
            trimmed.endsWith(".jpg", true) || trimmed.endsWith(".jpeg", true) -> "image/jpeg"
            trimmed.endsWith(".webp", true) -> "image/webp"
            trimmed.endsWith(".json", true) -> "application/json"
            trimmed.endsWith(".txt", true) -> "text/plain"
            else -> "*/*"
        }
        if (trimmed.startsWith("content://")) {
            return Uri.parse(trimmed) to mime
        }
        val file = File(trimmed)
        val uri = FileProvider.getUriForFile(context, "${context.packageName}.fileprovider", file)
        return uri to mime
    }

    private fun timestamp(): String = SimpleDateFormat("yyyyMMdd-HHmmss", Locale.US).format(Date())

    private fun resolve(requestId: String, payload: Any?) {
        val serialized = when (payload) {
            null -> "null"
            is String -> JSONObject.quote(payload)
            is Number, is Boolean -> payload.toString()
            else -> JSONObject.wrap(payload)?.toString() ?: "null"
        }
        webView.post {
            webView.evaluateJavascript("window.__imageStudioNativeResolve(${JSONObject.quote(requestId)}, $serialized)", null)
        }
    }

    private fun reject(requestId: String, message: String) {
        webView.post {
            webView.evaluateJavascript(
                "window.__imageStudioNativeReject(${JSONObject.quote(requestId)}, ${JSONObject.quote(message)})",
                null,
            )
        }
    }

    private fun emitNativeProgress(requestId: String, payload: Any?) {
        val serialized = when (payload) {
            null -> "null"
            is String -> JSONObject.quote(payload)
            is Number, is Boolean -> payload.toString()
            else -> JSONObject.wrap(payload)?.toString() ?: "null"
        }
        webView.post {
            webView.evaluateJavascript("window.__imageStudioNativeProgress?.(${JSONObject.quote(requestId)}, $serialized)", null)
        }
    }

    private class EarlyResolve : RuntimeException()
}
