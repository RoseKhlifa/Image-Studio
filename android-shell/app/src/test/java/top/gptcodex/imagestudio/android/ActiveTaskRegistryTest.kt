package top.gptcodex.imagestudio.android

import org.junit.Assert.assertEquals
import org.junit.Test

class ActiveTaskRegistryTest {
    @Test
    fun tracksDistinctConcurrentTasks() {
        val registry = ActiveTaskRegistry()

        assertEquals(1, registry.acquire("first"))
        assertEquals(2, registry.acquire("second"))
        assertEquals(1, registry.release("first"))
        assertEquals(0, registry.release("second"))
    }

    @Test
    fun duplicateAcquireAndReleaseAreIdempotent() {
        val registry = ActiveTaskRegistry()

        assertEquals(1, registry.acquire("same"))
        assertEquals(1, registry.acquire("same"))
        assertEquals(0, registry.release("same"))
        assertEquals(0, registry.release("same"))
    }

    @Test
    fun ignoresBlankAndUnknownTaskIds() {
        val registry = ActiveTaskRegistry()

        assertEquals(0, registry.acquire(""))
        assertEquals(0, registry.release("missing"))
        assertEquals(0, registry.count())
    }
}
