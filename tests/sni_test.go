package tests

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// buildMockTLSClientHello constructs a synthetic TLS 1.2/1.3 ClientHello containing the SNI extension
func buildMockTLSClientHello(serverName string) []byte {
	buf := new(bytes.Buffer)

	// TLS Record Header (5 bytes)
	buf.WriteByte(0x16)                   // ContentType: Handshake
	buf.Write([]byte{0x03, 0x01})         // Version: TLS 1.0 (Record layer)
	recordLenPos := buf.Len()
	buf.Write([]byte{0x00, 0x00})         // Placeholder for Record Length

	// Handshake Header (4 bytes)
	buf.WriteByte(0x01)                   // HandshakeType: ClientHello
	hsLenPos := buf.Len()
	buf.Write([]byte{0x00, 0x00, 0x00})   // Placeholder for Handshake Length

	// Client Version (2 bytes)
	buf.Write([]byte{0x03, 0x03})         // TLS 1.2
	// Random (32 bytes)
	buf.Write(make([]byte, 32))
	// Session ID (1 byte len + 0 bytes)
	buf.WriteByte(0x00)
	// Cipher Suites (2 bytes len + 2 bytes suite)
	buf.Write([]byte{0x00, 0x02, 0xc0, 0x2f})
	// Compression methods (1 byte len + 1 byte null)
	buf.Write([]byte{0x01, 0x00})

	// Extensions Section
	extBuf := new(bytes.Buffer)
	// SNI Extension: Type 0x0000
	extBuf.Write([]byte{0x00, 0x00}) // Type: server_name

	sniData := new(bytes.Buffer)
	// server_name_list length (2 bytes)
	sniListLen := 3 + len(serverName)
	_ = binary.Write(sniData, binary.BigEndian, uint16(sniListLen))
	sniData.WriteByte(0x00) // NameType: host_name (0)
	_ = binary.Write(sniData, binary.BigEndian, uint16(len(serverName)))
	sniData.WriteString(serverName)

	_ = binary.Write(extBuf, binary.BigEndian, uint16(sniData.Len()))
	extBuf.Write(sniData.Bytes())

	// Write Extensions length + data
	_ = binary.Write(buf, binary.BigEndian, uint16(extBuf.Len()))
	buf.Write(extBuf.Bytes())

	// Fill in lengths
	data := buf.Bytes()
	recordLen := len(data) - 5
	binary.BigEndian.PutUint16(data[recordLenPos:recordLenPos+2], uint16(recordLen))

	hsLen := len(data) - 9
	data[hsLenPos] = byte(hsLen >> 16)
	data[hsLenPos+1] = byte(hsLen >> 8)
	data[hsLenPos+2] = byte(hsLen)

	return data
}

func parseSNIFromBytes(data []byte) string {
	if len(data) < 43 || data[0] != 0x16 || data[5] != 0x01 {
		return ""
	}

	pos := 43
	if pos >= len(data) {
		return ""
	}
	sessionIDLen := int(data[pos])
	pos += 1 + sessionIDLen

	if pos+2 > len(data) {
		return ""
	}
	cipherLen := int(binary.BigEndian.Uint16(data[pos : pos+2]))
	pos += 2 + cipherLen

	if pos+1 > len(data) {
		return ""
	}
	compLen := int(data[pos])
	pos += 1 + compLen

	if pos+2 > len(data) {
		return ""
	}
	extLen := int(binary.BigEndian.Uint16(data[pos : pos+2]))
	pos += 2

	extEnd := pos + extLen
	if extEnd > len(data) {
		extEnd = len(data)
	}

	for pos+4 <= extEnd {
		extType := binary.BigEndian.Uint16(data[pos : pos+2])
		itemLen := int(binary.BigEndian.Uint16(data[pos+2 : pos+4]))
		pos += 4

		if pos+itemLen > extEnd {
			break
		}

		if extType == 0x0000 { // server_name
			if itemLen < 5 {
				break
			}
			nameLen := int(binary.BigEndian.Uint16(data[pos+3 : pos+5]))
			if pos+5+nameLen <= extEnd {
				return string(data[pos+5 : pos+5+nameLen])
			}
			break
		}
		pos += itemLen
	}
	return ""
}

func TestTLSClientHelloParser(t *testing.T) {
	domains := []string{"discord.com", "gateway.discord.gg", "cdn.discordapp.com"}

	for _, domain := range domains {
		ch := buildMockTLSClientHello(domain)
		parsed := parseSNIFromBytes(ch)
		if parsed != domain {
			t.Errorf("Expected parsed SNI %s, got %s", domain, parsed)
		}
	}
}

func TestTCPClientHelloSegmentation(t *testing.T) {
	ch := buildMockTLSClientHello("discord.com")
	splitPos := 2

	if len(ch) <= splitPos {
		t.Fatalf("ClientHello too short for split: %d", len(ch))
	}

	// First segment: first 2 bytes (\x16\x03)
	chunk1 := ch[:splitPos]
	// Second segment: remaining bytes
	chunk2 := ch[splitPos:]

	if len(chunk1) != 2 {
		t.Errorf("Expected chunk1 len 2, got %d", len(chunk1))
	}
	if chunk1[0] != 0x16 {
		t.Errorf("Expected chunk1[0] == 0x16, got 0x%02x", chunk1[0])
	}

	// Middlebox only inspecting chunk1 cannot parse TLS ClientHello or SNI
	if len(chunk1) >= 43 {
		t.Errorf("chunk1 should be truncated so DPI cannot parse SNI")
	}

	// Reassembled stream
	reassembled := append(chunk1, chunk2...)
	if !bytes.Equal(ch, reassembled) {
		t.Errorf("Reassembled stream does not equal original ClientHello")
	}
}
