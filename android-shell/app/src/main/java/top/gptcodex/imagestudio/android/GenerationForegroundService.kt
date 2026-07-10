package top.gptcodex.imagestudio.android

import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.Context
import android.content.Intent
import android.content.pm.ServiceInfo
import android.os.Build
import android.os.IBinder
import android.os.PowerManager
import android.util.Log
import androidx.core.app.NotificationCompat
import androidx.core.app.ServiceCompat
import androidx.core.content.ContextCompat

class GenerationForegroundService : Service() {
    private var wakeLock: PowerManager.WakeLock? = null

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        createNotificationChannel()
        val taskCount = activeTasks.count()
        val notification = NotificationCompat.Builder(this, notificationChannelId)
            .setSmallIcon(R.drawable.ic_notification)
            .setContentTitle(getString(R.string.background_task_notification_title))
            .setContentText(resources.getQuantityString(R.plurals.background_task_notification_text, taskCount, taskCount))
            .setContentIntent(openAppPendingIntent())
            .setCategory(NotificationCompat.CATEGORY_PROGRESS)
            .setPriority(NotificationCompat.PRIORITY_LOW)
            .setOngoing(true)
            .setOnlyAlertOnce(true)
            .setProgress(0, 0, true)
            .setForegroundServiceBehavior(NotificationCompat.FOREGROUND_SERVICE_IMMEDIATE)
            .build()
        val serviceType = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
            ServiceInfo.FOREGROUND_SERVICE_TYPE_DATA_SYNC
        } else {
            0
        }
        ServiceCompat.startForeground(this, notificationId, notification, serviceType)

        if (taskCount == 0) {
            ServiceCompat.stopForeground(this, ServiceCompat.STOP_FOREGROUND_REMOVE)
            stopSelf(startId)
            return START_NOT_STICKY
        }

        refreshWakeLock()
        return START_NOT_STICKY
    }

    override fun onDestroy() {
        releaseWakeLock()
        super.onDestroy()
    }

    private fun createNotificationChannel() {
        val channel = NotificationChannel(
            notificationChannelId,
            getString(R.string.background_task_channel_name),
            NotificationManager.IMPORTANCE_LOW,
        ).apply {
            description = getString(R.string.background_task_channel_description)
            setShowBadge(false)
        }
        getSystemService(NotificationManager::class.java).createNotificationChannel(channel)
    }

    private fun openAppPendingIntent(): PendingIntent {
        val intent = Intent(this, MainActivity::class.java).apply {
            addFlags(Intent.FLAG_ACTIVITY_CLEAR_TOP or Intent.FLAG_ACTIVITY_SINGLE_TOP)
        }
        return PendingIntent.getActivity(
            this,
            0,
            intent,
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
        )
    }

    private fun refreshWakeLock() {
        releaseWakeLock()
        val powerManager = getSystemService(Context.POWER_SERVICE) as PowerManager
        wakeLock = powerManager.newWakeLock(
            PowerManager.PARTIAL_WAKE_LOCK,
            "$packageName:active-image-task",
        ).apply {
            setReferenceCounted(false)
            acquire(wakeLockTimeoutMs)
        }
    }

    private fun releaseWakeLock() {
        wakeLock?.let { lock ->
            if (lock.isHeld) lock.release()
        }
        wakeLock = null
    }

    companion object {
        private const val notificationChannelId = "image_generation_tasks"
        private const val notificationId = 4107
        private const val wakeLockTimeoutMs = 10L * 60L * 1000L
        private val activeTasks = ActiveTaskRegistry()

        fun acquire(context: Context, taskId: String) {
            activeTasks.acquire(taskId)
            val appContext = context.applicationContext
            try {
                ContextCompat.startForegroundService(
                    appContext,
                    Intent(appContext, GenerationForegroundService::class.java),
                )
            } catch (error: RuntimeException) {
                activeTasks.release(taskId)
                Log.w("ImageStudioKeepAlive", "Unable to start foreground service", error)
            }
        }

        fun release(context: Context, taskId: String) {
            val remaining = activeTasks.release(taskId)
            val appContext = context.applicationContext
            if (remaining == 0) {
                appContext.stopService(Intent(appContext, GenerationForegroundService::class.java))
                return
            }
            try {
                ContextCompat.startForegroundService(
                    appContext,
                    Intent(appContext, GenerationForegroundService::class.java),
                )
            } catch (error: RuntimeException) {
                Log.w("ImageStudioKeepAlive", "Unable to update foreground service", error)
            }
        }
    }
}
