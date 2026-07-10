package top.gptcodex.imagestudio.android

import java.io.ByteArrayInputStream
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Assert.assertThrows
import org.junit.Test

class AndroidBinaryResponseTest {
    @Test
    fun `reads a binary response within the configured limit`() {
        val bytes = byteArrayOf(0x01, 0x02, 0x03, 0x04)
        assertArrayEquals(bytes, readLimitedBytes(ByteArrayInputStream(bytes), bytes.size))
    }

    @Test
    fun `rejects a binary response over the configured limit`() {
        assertThrows(IllegalStateException::class.java) {
            readLimitedBytes(ByteArrayInputStream(ByteArray(5)), 4)
        }
    }

    @Test
    fun `host hard caps caller requested binary responses`() {
        val hardMax = 50L * 1024L * 1024L
        assertEquals(hardMax.toInt(), boundedBinaryResponseLimit(Long.MAX_VALUE, hardMax))
        assertEquals(1024, boundedBinaryResponseLimit(1024, hardMax))
        assertEquals(hardMax.toInt(), boundedBinaryResponseLimit(0, hardMax))
    }
}
