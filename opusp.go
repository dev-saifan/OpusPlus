package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

type OpusPlusFooter struct {
	Version   uint8
	MetaSize  uint32
	CoverSize uint32
	AudioSize uint64
}

type TrackMetadata struct {
	Title       string            `json:"title"`
	Artist      string            `json:"artist"`
	Album       string            `json:"album"`
	Year        int               `json:"year,omitempty"`
	Genre       string            `json:"genre,omitempty"`
	BitrateKbps int               `json:"bitrate_kbps,omitempty"`
	SampleRate  int               `json:"sample_rate,omitempty"`
	Lyrics      string            `json:"lyrics,omitempty"`
	AudioSHA256 string            `json:"audio_sha256"`
	ExtraTags   map[string]string `json:"extra_tags,omitempty"`
}

const magicSignature = "OPUS+"
const footerTotalSize = 22

func PackOpusPlus(audioPath, coverPath, outPath string, meta TrackMetadata) error {
	audioBytes, err := os.ReadFile(audioPath)
	if err != nil {
		return err
	}

	hash := sha256.Sum256(audioBytes)
	meta.AudioSHA256 = hex.EncodeToString(hash[:])

	metaBytes, err := json.Marshal(meta)
	if err != nil {
		return err
	}

	var coverBytes []byte
	if coverPath != "" {
		coverBytes, err = os.ReadFile(coverPath)
		if err != nil {
			return err
		}
	}

	footer := OpusPlusFooter{
		Version:   2,
		MetaSize:  uint32(len(metaBytes)),
		CoverSize: uint32(len(coverBytes)),
		AudioSize: uint64(len(audioBytes)),
	}

	buf := new(bytes.Buffer)
	buf.Write(audioBytes)
	buf.Write(metaBytes)
	buf.Write(coverBytes)
	buf.WriteString(magicSignature)
	if err := binary.Write(buf, binary.LittleEndian, footer); err != nil {
		return err
	}

	return os.WriteFile(outPath, buf.Bytes(), 0644)
}

func ReadOpusPlus(filePath string) (TrackMetadata, []byte, []byte, error) {
	var meta TrackMetadata
	data, err := os.ReadFile(filePath)
	if err != nil {
		return meta, nil, nil, err
	}

	if len(data) < footerTotalSize {
		return meta, nil, nil, fmt.Errorf("file too small")
	}

	footerStart := len(data) - footerTotalSize
	magicBytes := data[footerStart : footerStart+5]
	if string(magicBytes) != magicSignature {
		return meta, nil, nil, fmt.Errorf("invalid .opus+ signature")
	}

	var footer OpusPlusFooter
	if err := binary.Read(bytes.NewReader(data[footerStart+5:]), binary.LittleEndian, &footer); err != nil {
		return meta, nil, nil, err
	}

	audioEnd := uint64(footerStart) - uint64(footer.CoverSize) - uint64(footer.MetaSize)
	metaEnd := audioEnd + uint64(footer.MetaSize)
	coverEnd := metaEnd + uint64(footer.CoverSize)

	audio := data[:audioEnd]
	metaBytes := data[audioEnd:metaEnd]
	cover := data[metaEnd:coverEnd]

	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		return meta, nil, nil, err
	}

	return meta, cover, audio, nil
}

func main() {
	args := os.Args[1:]
	if len(args) < 1 {
		fmt.Println("Usage:")
		fmt.Println("  opusp.exe pack -in song.opus -out song.opus+ -title \"Title\" -artist \"Artist\"")
		fmt.Println("  opusp.exe info song.opus+")
		os.Exit(1)
	}

	command := args[:1][0]
	subArgs := args[1:]

	switch command {
	case "pack":
		packCmd := flag.NewFlagSet("pack", flag.ExitOnError)
		inFile := packCmd.String("in", "", "Input .opus file (required)")
		coverFile := packCmd.String("cover", "", "Cover image (.jpg or .png)")
		outFile := packCmd.String("out", "output.opus+", "Output .opus+ file")
		title := packCmd.String("title", "Unknown Title", "Track title")
		artist := packCmd.String("artist", "Unknown Artist", "Artist name")
		album := packCmd.String("album", "", "Album name")
		year := packCmd.Int("year", 0, "Release year")
		genre := packCmd.String("genre", "", "Genre")

		packCmd.Parse(subArgs)

		if *inFile == "" {
			fmt.Println("Error: please specify an input file using -in")
			os.Exit(1)
		}

		meta := TrackMetadata{
			Title:  *title,
			Artist: *artist,
			Album:  *album,
			Year:   *year,
			Genre:  *genre,
		}

		if err := PackOpusPlus(*inFile, *coverFile, *outFile, meta); err != nil {
			fmt.Println("Failed to pack:", err)
			os.Exit(1)
		}
		fmt.Printf("Successfully packed '%s' -> '%s'\n", *inFile, *outFile)

	case "info":
		if len(subArgs) < 1 {
			fmt.Println("Error: provide a .opus+ file path")
			os.Exit(1)
		}
		targetFile := subArgs[:1][0]
		meta, cover, audio, err := ReadOpusPlus(targetFile)
		if err != nil {
			fmt.Println("Error reading file:", err)
			os.Exit(1)
		}
		fmt.Printf("Title:   %s\n", meta.Title)
		fmt.Printf("Artist:  %s\n", meta.Artist)
		fmt.Printf("Album:   %s\n", meta.Album)
		fmt.Printf("Year:    %d\n", meta.Year)
		fmt.Printf("Genre:   %s\n", meta.Genre)
		fmt.Printf("SHA-256: %s\n", meta.AudioSHA256)
		fmt.Printf("Cover:   %d bytes\n", len(cover))
		fmt.Printf("Audio:   %d bytes\n", len(audio))

	default:
		fmt.Println("Unknown command. Use 'pack' or 'info'.")
	}
}
