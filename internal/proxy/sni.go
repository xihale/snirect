package proxy

import (
	"encoding/binary"
	"fmt"
)

// ParseSNI extracts the Server Name Indication (SNI) from a TLS Client Hello packet.
func ParseSNI(data []byte) (string, error) {
	pos, err := findSNIPos(data)
	if err != nil {
		return "", err
	}
	if pos+3 > len(data) {
		return "", fmt.Errorf("overflow")
	}
	nameLen := int(binary.BigEndian.Uint16(data[pos+1 : pos+3]))
	if pos+3+nameLen > len(data) {
		return "", fmt.Errorf("overflow")
	}
	return string(data[pos+3 : pos+3+nameLen]), nil
}

func findSNIPos(data []byte) (int, error) {
	if len(data) < 43 {
		return 0, fmt.Errorf("short")
	}
	// Skip Type(1) + Length(3) + Version(2) + Random(32)
	pos := 5 + 1 + 32
	if pos >= len(data) {
		return 0, fmt.Errorf("overflow")
	}
	sessionIDLen := int(data[pos])
	pos += 1 + sessionIDLen
	if pos+2 > len(data) {
		return 0, fmt.Errorf("overflow")
	}
	cipherSuitesLen := int(binary.BigEndian.Uint16(data[pos : pos+2]))
	pos += 2 + cipherSuitesLen
	if pos+1 > len(data) {
		return 0, fmt.Errorf("overflow")
	}
	compMethodsLen := int(data[pos])
	pos += 1 + compMethodsLen
	if pos+2 > len(data) {
		return 0, fmt.Errorf("overflow")
	}
	extLen := int(binary.BigEndian.Uint16(data[pos : pos+2]))
	pos += 2
	end := pos + extLen
	for pos+4 <= end && pos+4 <= len(data) {
		extType := binary.BigEndian.Uint16(data[pos : pos+2])
		extSize := int(binary.BigEndian.Uint16(data[pos+2 : pos+4]))
		pos += 4
		if extType == 0x00 { // Server Name
			if pos+2 > len(data) {
				return 0, fmt.Errorf("overflow")
			}
			// Skip Server Name List Length(2) + Type(1)
			return pos + 2, nil
		}
		pos += extSize
	}
	return 0, fmt.Errorf("not found")
}

// TryModifySNI attempts to replace the SNI in a Client Hello packet.
// This only works if the new SNI has the same length as the old one.
func TryModifySNI(data []byte, old, new string) []byte {
	pos, err := findSNIPos(data)
	if err != nil {
		return nil
	}
	nameLen := int(binary.BigEndian.Uint16(data[pos+1 : pos+3]))
	if len(new) == nameLen {
		newData := make([]byte, len(data))
		copy(newData, data)
		copy(newData[pos+3:], []byte(new))
		return newData
	}
	return nil
}
