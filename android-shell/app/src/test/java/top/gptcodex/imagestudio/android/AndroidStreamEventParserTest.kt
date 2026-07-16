package top.gptcodex.imagestudio.android

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class AndroidStreamEventParserTest {
    @Test
    fun `extracts final Responses image result`() {
        val result = extractNativeHttpStreamResult(
            """data: {"type":"response.output_item.done","item":{"type":"image_generation_call","result":"b64-final","revised_prompt":"better prompt"}}""",
        )

        assertNotNull(result)
        assertEquals("b64-final", result?.imageB64)
        assertEquals("better prompt", result?.revisedPrompt)
        assertEquals("final", result?.sourceEvent)
    }

    @Test
    fun `extracts final Images API result`() {
        val result = extractNativeHttpStreamResult(
            """data: {"type":"image_generation.completed","b64_json":"b64-images"}""",
        )

        assertNotNull(result)
        assertEquals("b64-images", result?.imageB64)
        assertEquals("", result?.revisedPrompt)
        assertEquals("images_api", result?.sourceEvent)
    }

    @Test
    fun `skips empty keepalives before standard Images API result`() {
        assertNull(extractNativeHttpStreamResult(""))
        assertNull(extractNativeHttpStreamResult("data:"))
        assertNull(extractNativeHttpStreamResult("data:{}"))
        assertNull(extractNativeHttpStreamResult("data: {\"data\":[]}"))

        val result = extractNativeHttpStreamResult(
            """data:{"data":[{"b64_json":"b64-final","revised_prompt":"kept alive"}]}""",
        )
        assertNotNull(result)
        assertEquals("b64-final", result?.imageB64)
        assertEquals("kept alive", result?.revisedPrompt)
        assertEquals("images_api", result?.sourceEvent)
    }

    @Test
    fun `extracts standard Images API result from raw JSON stream line`() {
        val result = extractNativeHttpStreamResult(
            """{"data":[{"b64_json":"b64-final"}]}""",
        )
        assertEquals("b64-final", result?.imageB64)
    }

    @Test
    fun `builds progress payload for Responses partial image`() {
        val payload = buildNativeHttpStreamProgressPayload(
            """data: {"type":"response.image_generation_call.partial_image","partial_image_b64":"b64-preview","revised_prompt":"preview prompt","partial_image_index":2}""",
        ) as Map<*, *>

        val event = payload["event"] as Map<*, *>
        assertEquals("response.image_generation_call.partial_image", event["type"])
        assertEquals("b64-preview", event["partial_image_b64"])
        assertEquals("preview prompt", event["revised_prompt"])
        assertEquals(2, event["partial_image_index"])
    }

    @Test
    fun `builds progress payload for Images API partial image`() {
        val payload = buildNativeHttpStreamProgressPayload(
            """data: {"type":"image_edit.partial_image","b64_json":"b64-preview","partial_image_index":1}""",
        ) as Map<*, *>

        val event = payload["event"] as Map<*, *>
        assertEquals("image_edit.partial_image", event["type"])
        assertEquals("b64-preview", event["b64_json"])
        assertEquals(1, event["partial_image_index"])
    }

    @Test
    fun `suppresses final result from progress payload once image is complete`() {
        val payload = buildNativeHttpStreamProgressPayload(
            """data: {"type":"response.output_item.done","item":{"type":"image_generation_call","result":"b64-final"}}""",
        )

        assertNull(payload)
    }

    @Test
    fun `passes through non data lines for raw log progress`() {
        val payload = buildNativeHttpStreamProgressPayload("event: heartbeat") as Map<*, *>
        assertEquals("event: heartbeat", payload["line"])
    }

    @Test
    fun `returns line fallback for malformed JSON`() {
        val payload = buildNativeHttpStreamProgressPayload("data: {not-json}") as Map<*, *>
        assertTrue(payload.containsKey("line"))
    }
}
