package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"strings"
	"time"
)

type MinecraftStatus struct {
	Online      bool   `json:"online"`
	LatencyMS   int64  `json:"latency_ms,omitempty"`
	Players     int    `json:"players,omitempty"`
	MaxPlayers  int    `json:"max_players,omitempty"`
	Version     string `json:"version,omitempty"`
	Description string `json:"description,omitempty"`
	Maintenance bool   `json:"maintenance"`
}

func pingMinecraft(address string, timeout time.Duration) MinecraftStatus {
	started := time.Now()
	connection, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return MinecraftStatus{Online: false}
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(timeout))
	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		return MinecraftStatus{Online: false}
	}
	port := 25565
	_, _ = fmtSscanf(portText, &port)

	var handshake bytes.Buffer
	writeVarInt(&handshake, 0)
	writeVarInt(&handshake, 774)
	writeString(&handshake, host)
	_ = binary.Write(&handshake, binary.BigEndian, uint16(port))
	writeVarInt(&handshake, 1)
	if err := writePacket(connection, handshake.Bytes()); err != nil {
		return MinecraftStatus{Online: false}
	}
	if err := writePacket(connection, []byte{0}); err != nil {
		return MinecraftStatus{Online: false}
	}

	reader := bufio.NewReader(connection)
	if _, err := readVarInt(reader); err != nil {
		return MinecraftStatus{Online: false}
	}
	packetID, err := readVarInt(reader)
	if err != nil || packetID != 0 {
		return MinecraftStatus{Online: false}
	}
	length, err := readVarInt(reader)
	if err != nil || length < 1 || length > 1<<20 {
		return MinecraftStatus{Online: false}
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return MinecraftStatus{Online: false}
	}

	var response struct {
		Version struct {
			Name string `json:"name"`
		} `json:"version"`
		Players struct {
			Online int `json:"online"`
			Max    int `json:"max"`
		} `json:"players"`
		Description any `json:"description"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		return MinecraftStatus{Online: false}
	}
	description := ""
	switch value := response.Description.(type) {
	case string:
		description = value
	case map[string]any:
		if text, ok := value["text"].(string); ok {
			description = text
		}
	}
	return MinecraftStatus{
		Online: true, LatencyMS: time.Since(started).Milliseconds(),
		Players: response.Players.Online, MaxPlayers: response.Players.Max,
		Version: response.Version.Name, Description: description,
	}
}

func rconCommand(address, password, command string, timeout time.Duration) (string, error) {
	if password == "" {
		return "", errors.New("RCON password unavailable")
	}
	connection, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return "", err
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(timeout))

	if err := writeRCONPacket(connection, 1, 3, password); err != nil {
		return "", err
	}
	id, _, _, err := readRCONPacket(connection)
	if err != nil || id == -1 {
		return "", errors.New("RCON authentication failed")
	}
	if err := writeRCONPacket(connection, 2, 2, command); err != nil {
		return "", err
	}
	id, _, response, err := readRCONPacket(connection)
	if err != nil || id != 2 {
		return "", errors.New("RCON command failed")
	}
	return strings.TrimSpace(response), nil
}

func writeRCONPacket(writer io.Writer, id, packetType int32, body string) error {
	if len(body) > 4096 {
		return errors.New("RCON packet is too large")
	}
	length := int32(10 + len(body))
	var packet bytes.Buffer
	_ = binary.Write(&packet, binary.LittleEndian, length)
	_ = binary.Write(&packet, binary.LittleEndian, id)
	_ = binary.Write(&packet, binary.LittleEndian, packetType)
	packet.WriteString(body)
	packet.Write([]byte{0, 0})
	_, err := writer.Write(packet.Bytes())
	return err
}

func readRCONPacket(reader io.Reader) (int32, int32, string, error) {
	var length int32
	if err := binary.Read(reader, binary.LittleEndian, &length); err != nil {
		return 0, 0, "", err
	}
	if length < 10 || length > 64*1024 {
		return 0, 0, "", errors.New("invalid RCON packet size")
	}
	packet := make([]byte, length)
	if _, err := io.ReadFull(reader, packet); err != nil {
		return 0, 0, "", err
	}
	id := int32(binary.LittleEndian.Uint32(packet[0:4]))
	packetType := int32(binary.LittleEndian.Uint32(packet[4:8]))
	return id, packetType, string(packet[8 : len(packet)-2]), nil
}

func writePacket(writer io.Writer, payload []byte) error {
	var framed bytes.Buffer
	writeVarInt(&framed, len(payload))
	framed.Write(payload)
	_, err := writer.Write(framed.Bytes())
	return err
}

func writeString(writer io.Writer, value string) {
	writeVarInt(writer, len(value))
	_, _ = io.WriteString(writer, value)
}

func writeVarInt(writer io.Writer, value int) {
	unsigned := uint32(value)
	for {
		if unsigned&^0x7F == 0 {
			_, _ = writer.Write([]byte{byte(unsigned)})
			return
		}
		_, _ = writer.Write([]byte{byte(unsigned&0x7F | 0x80)})
		unsigned >>= 7
	}
}

func readVarInt(reader io.ByteReader) (int, error) {
	value := 0
	position := 0
	for {
		current, err := reader.ReadByte()
		if err != nil {
			return 0, err
		}
		value |= int(current&0x7F) << position
		if current&0x80 == 0 {
			return value, nil
		}
		position += 7
		if position >= 35 {
			return 0, errors.New("VarInt is too large")
		}
	}
}

func fmtSscanf(value string, destination *int) (int, error) {
	var parsed int
	for _, character := range value {
		if character < '0' || character > '9' {
			return 0, errors.New("invalid number")
		}
		parsed = parsed*10 + int(character-'0')
	}
	*destination = parsed
	return 1, nil
}
