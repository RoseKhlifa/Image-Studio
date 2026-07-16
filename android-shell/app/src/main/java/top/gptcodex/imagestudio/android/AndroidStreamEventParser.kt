package top.gptcodex.imagestudio.android

import org.json.JSONObject

internal data class NativeHttpStreamResultSnapshot(
    val imageB64: String,
    val revisedPrompt: String,
    val sourceEvent: String,
)

private fun parseNativeHttpStreamEvent(line: String): JSONObject? {
    val trimmed = line.trim()
    if (trimmed.isBlank() || trimmed.startsWith(":")) return null
    val payload = if (trimmed.startsWith("data:")) {
        trimmed.removePrefix("data:").trim()
    } else {
        trimmed
    }
    if (payload.isBlank() || payload == "[DONE]") return null
    return try {
        JSONObject(payload)
    } catch (_: Exception) {
        null
    }
}

private fun extractNativeHttpStreamResult(event: JSONObject): NativeHttpStreamResultSnapshot? {
    val firstImage = event.optJSONArray("data")?.optJSONObject(0)
    if (firstImage != null) {
        val imageB64 = firstImage.optString("b64_json")
        if (imageB64.isNotBlank()) {
            return NativeHttpStreamResultSnapshot(
                imageB64 = imageB64,
                revisedPrompt = firstImage.optString("revised_prompt"),
                sourceEvent = "images_api",
            )
        }
    }
    return when (event.optString("type")) {
        "response.output_item.done" -> {
            val item = event.optJSONObject("item")
            if (item?.optString("type") == "image_generation_call") {
                val result = item.optString("result")
                if (result.isNotBlank()) {
                    NativeHttpStreamResultSnapshot(
                        imageB64 = result,
                        revisedPrompt = item.optString("revised_prompt"),
                        sourceEvent = "final",
                    )
                } else null
            } else null
        }
        "image_generation.completed", "image_edit.completed" -> {
            val result = event.optString("b64_json")
            if (result.isNotBlank()) {
                NativeHttpStreamResultSnapshot(
                    imageB64 = result,
                    revisedPrompt = "",
                    sourceEvent = "images_api",
                )
            } else null
        }
        else -> null
    }
}

internal fun extractNativeHttpStreamResult(line: String): NativeHttpStreamResultSnapshot? {
    return parseNativeHttpStreamEvent(line)?.let(::extractNativeHttpStreamResult)
}

internal fun buildNativeHttpStreamProgressPayload(line: String): Any? {
    val event = parseNativeHttpStreamEvent(line)
    if (event == null) {
        return mapOf("line" to line)
    }
    return try {
        when (event.optString("type")) {
            "response.image_generation_call.partial_image" -> mapOf(
                "event" to mapOf(
                    "type" to "response.image_generation_call.partial_image",
                    "partial_image_b64" to event.optString("partial_image_b64"),
                    "revised_prompt" to event.optString("revised_prompt"),
                    "partial_image_index" to if (event.has("partial_image_index")) event.optInt("partial_image_index", -1) else -1,
                ),
            )
            "image_generation.partial_image", "image_edit.partial_image" -> mapOf(
                "event" to mapOf(
                    "type" to event.optString("type"),
                    "b64_json" to event.optString("b64_json"),
                    "partial_image_index" to if (event.has("partial_image_index")) event.optInt("partial_image_index", -1) else -1,
                ),
            )
            "response.output_item.done" -> {
                val item = event.optJSONObject("item")
                if (item?.optString("type") == "image_generation_call" && item.optString("result").isNotBlank()) {
                    null
                } else {
                    mapOf("line" to line)
                }
            }
            "image_generation.completed", "image_edit.completed" -> {
                if (event.optString("b64_json").isNotBlank()) null else mapOf("line" to line)
            }
            else -> if (extractNativeHttpStreamResult(event) != null) null else mapOf("line" to line)
        }
    } catch (_: Exception) {
        mapOf("line" to line)
    }
}
