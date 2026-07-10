package top.gptcodex.imagestudio.android

internal class ActiveTaskRegistry {
    private val taskIds = linkedSetOf<String>()

    @Synchronized
    fun acquire(taskId: String): Int {
        if (taskId.isNotBlank()) taskIds += taskId
        return taskIds.size
    }

    @Synchronized
    fun release(taskId: String): Int {
        if (taskId.isNotBlank()) taskIds -= taskId
        return taskIds.size
    }

    @Synchronized
    fun count(): Int = taskIds.size
}
