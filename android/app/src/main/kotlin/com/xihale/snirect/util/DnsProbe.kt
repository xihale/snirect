package com.xihale.snirect.util

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.io.ByteArrayOutputStream
import java.net.DatagramPacket
import java.net.DatagramSocket
import java.net.InetAddress
import java.net.InetSocketAddress
import java.net.Socket
import java.net.URL
import javax.net.ssl.HttpsURLConnection

/**
 * One-shot connectivity probe for a configured nameserver entry.
 *
 * The strategy follows the server's scheme so the test exercises the same
 * path the engine would use:
 * - `https://…` → RFC 8484 DoH POST, any DNS answer counts as reachable
 * - `tls://…`  → TCP connect to :853 (the TLS handshake belongs to the engine)
 * - bare host  → plain UDP DNS query to :53
 *
 * Returns the round-trip in milliseconds on success, or the failure reason.
 */
object DnsProbe {
    // A fixed, guaranteed-to-exist name; we only care that the server answers.
    private const val QUERY_NAME = "www.example.com"

    suspend fun test(server: String, timeoutMs: Int = 3000): Result<Long> =
        withContext(Dispatchers.IO) {
            runCatching {
                when {
                    server.startsWith("https://") -> testDoh(server, timeoutMs)
                    server.startsWith("tls://") -> testTcp(
                        hostPort = server.removePrefix("tls://"),
                        defaultPort = 853,
                        timeoutMs = timeoutMs
                    )
                    else -> testUdp(
                        hostPort = server.removePrefix("udp://"),
                        defaultPort = 53,
                        timeoutMs = timeoutMs
                    )
                }
            }
        }

    /** Minimal A-record query bytes for [QUERY_NAME]. */
    private fun buildQuery(id: Int): ByteArray {
        val out = ByteArrayOutputStream()
        // Header: id, flags = recursion desired, qdcount = 1, rest zero.
        out.write(id shr 8); out.write(id)
        out.write(0x01); out.write(0x00)
        out.write(0); out.write(1)
        out.write(0); out.write(0)
        out.write(0); out.write(0)
        out.write(0); out.write(0)
        // QNAME labels.
        QUERY_NAME.split('.').forEach { label ->
            out.write(label.length)
            out.write(label.toByteArray(Charsets.US_ASCII))
        }
        out.write(0)
        // QTYPE = A, QCLASS = IN.
        out.write(0); out.write(1)
        out.write(0); out.write(1)
        return out.toByteArray()
    }

    private fun testUdp(hostPort: String, defaultPort: Int, timeoutMs: Int): Long {
        val (host, port) = splitHostPort(hostPort, defaultPort)
        val query = buildQuery(id = (System.nanoTime() and 0xFFFF).toInt())
        DatagramSocket().use { socket ->
            socket.soTimeout = timeoutMs
            val address = InetAddress.getByName(host)
            val start = System.nanoTime()
            socket.send(DatagramPacket(query, query.size, address, port))
            val buffer = ByteArray(512)
            socket.receive(DatagramPacket(buffer, buffer.size))
            return (System.nanoTime() - start) / 1_000_000
        }
    }

    private fun testTcp(hostPort: String, defaultPort: Int, timeoutMs: Int): Long {
        val (host, port) = splitHostPort(hostPort, defaultPort)
        Socket().use { socket ->
            val start = System.nanoTime()
            socket.connect(InetSocketAddress(host, port), timeoutMs)
            return (System.nanoTime() - start) / 1_000_000
        }
    }

    private fun testDoh(server: String, timeoutMs: Int): Long {
        val query = buildQuery(id = (System.nanoTime() and 0xFFFF).toInt())
        val connection = URL(server).openConnection() as HttpsURLConnection
        try {
            connection.connectTimeout = timeoutMs
            connection.readTimeout = timeoutMs
            connection.requestMethod = "POST"
            connection.doOutput = true
            connection.setRequestProperty("Content-Type", "application/dns-message")
            connection.setRequestProperty("Accept", "application/dns-message")
            val start = System.nanoTime()
            connection.outputStream.use { it.write(query) }
            val code = connection.responseCode
            // Any well-formed DNS message back (even NXDOMAIN) proves the
            // server is reachable and answering queries.
            if (code != 200) error("HTTP $code")
            val body = connection.inputStream.use { it.readBytes() }
            if (body.size < 12) error("short response (${body.size} bytes)")
            return (System.nanoTime() - start) / 1_000_000
        } finally {
            connection.disconnect()
        }
    }

    private fun splitHostPort(hostPort: String, defaultPort: Int): Pair<String, Int> {
        val value = hostPort.trim().removePrefix("//")
        if (!value.contains(':')) return value to defaultPort
        val index = value.lastIndexOf(':')
        val port = value.substring(index + 1).toIntOrNull() ?: defaultPort
        return value.substring(0, index) to port
    }
}
