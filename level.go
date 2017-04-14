package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/draw"
	"io"
	"os"
	"strconv"

	gl "github.com/go-gl/gl/v3.1/gles2"

	bmp "golang.org/x/image/bmp"
)

const LevelSize = 1024
const TileSize = 16

type tileMap [LevelSize * LevelSize]uint8

func (t *tileMap) Index(x, y uint) uint {
	return y*LevelSize + x
}

type Level struct {
	tiles *tileMap

	texture uint32
}

func (l *Level) TextureID() uint32 {
	return l.texture
}

func (l *Level) Texel(tile uint8) (x float32, y float32) {
	if tile == 0 {
		panic("Can't get texel for tile index 0.")
	}

	nHorizontal := 304 / TileSize
	nVertical := 160 / TileSize

	tile--
	y = float32(int(tile)/nHorizontal) / float32(nVertical)
	x = float32(int(tile)%nHorizontal) / float32(nHorizontal)

	return
}

func (l *Level) Tile(x, y uint) uint8 {
	if x >= LevelSize || y >= LevelSize {
		panic("x(" + strconv.Itoa(int(x)) + ") and y(" + strconv.Itoa(int(y)) + ") should be < 1024.")
	}

	return l.tiles[l.tiles.Index(x, y)]
}

func NewLevelFromFile(file string) (*Level, error) {
	var fh *os.File
	var err error
	if fh, err = os.Open(file); err != nil {
		return nil, err
	}
	defer fh.Close()

	lvl := &Level{
		tiles: &tileMap{},
	}

	fh.Seek(0, 0)
	img, err := bmp.Decode(fh)
	if img.Bounds().Dx() != 304 || img.Bounds().Dy() != 160 {
		return nil, errors.New("Level tilemap should be 304x160")
	}
	lvl.texture = createTexture(img)
	if err != nil {
		return nil, err
	}

	tileOffset := startOfTiles(fh)
	fh.Seek(int64(tileOffset), 0)
	readTiles(fh, lvl.tiles)

	return lvl, nil
}

func createTexture(img image.Image) uint32 {
	rgba := image.NewRGBA(img.Bounds())
	draw.Draw(rgba, rgba.Bounds(), img, image.Pt(0, 0), draw.Src)

	var handle uint32
	gl.GenTextures(1, &handle)

	gl.BindTexture(gl.TEXTURE_2D, handle)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, int32(rgba.Bounds().Dx()), int32(rgba.Bounds().Dy()), 0, gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(rgba.Pix))

	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)

	return handle
}

func readTiles(r io.ReadSeeker, tiles *tileMap) {
	buf := [4]byte{}

	n, err := r.Read(buf[:])
	for n != 0 && err == nil {
		tileData := binary.LittleEndian.Uint32(buf[:])
		var tile = tileData >> 24
		var x = tileData << 20 >> 20
		var y = tileData << 8 >> 20

		tiles[tiles.Index(uint(x), uint(y))] = uint8(tile)

		n, err = r.Read(buf[:])
	}
}

func startOfTiles(r io.ReadSeeker) uint32 {
	current, _ := r.Seek(0, 1)
	defer r.Seek(current, 0)

	// Skip BM bytes.
	r.Seek(2, 0)

	sizeBuf := [4]byte{}
	r.Read(sizeBuf[:])

	var size uint32
	binary.Read(bytes.NewBuffer(sizeBuf[:]), binary.LittleEndian, &size)

	return size
}
