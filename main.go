package main

import (
	"fmt"
	"math"
	"runtime"
	"time"
	"unsafe"

	//	_ "net/http/pprof"

	gl "github.com/go-gl/gl/v3.1/gles2"
	"github.com/go-gl/glfw/v3.2/glfw"
	"github.com/trtstm/goss/ships"
)

func init() {
	// This is needed to arrange that main() runs on main thread.
	// See documentation for functions that are only allowed to be called from the main thread.
	runtime.LockOSThread()
}

type ScreenConfig struct {
	ResX uint
	ResY uint
}

type Config struct {
	Screen ScreenConfig
}

var config = Config{
	Screen: ScreenConfig{
		ResX: 1024,
		ResY: 800,
	},
}

var mapVertexShaderSrc = `
attribute vec4 a_pos;
attribute vec2 a_tex;

varying vec3 v_color;
varying vec2 v_tex;

uniform float time;

void main()
{
    float x = 2.0*a_pos.x/1024.0 - 1.0;
    float y = -2.0*a_pos.y/800.0 + 1.0;
    gl_Position = vec4(x, y, a_pos.z, a_pos.w);
	v_tex = a_tex;
    //v_color = vec3(1.0, 0.0, 0.0);
	//gl_Position = a_pos;
}
`

var mapFragmentShaderSrc = `
precision mediump float; // Precision required in gles.

varying vec3 v_color;
varying vec2 v_tex;

uniform float time;
uniform vec2 u_mouse;

uniform sampler2D u_mapTexture;

void main()
{
    float x = gl_FragCoord.x/1024.0;
    float y = -((gl_FragCoord.y/800.0)-1.0);

    float mouseX = u_mouse.x/1024.0;
    float mouseY = u_mouse.y/800.0;

    float dist = sqrt((mouseX - x)*(mouseX-x) + (mouseY - y)*(mouseY-y));
    gl_FragColor = texture2D(u_mapTexture, v_tex);
	//gl_FragColor = vec4(1.0, 0.0, 0.0, 1.0);
}
`

var mapVao uint32
var mapVbo uint32
var mapProgram uint32
var nMapVertecies uint32
var mapPosAttribute int32
var mapTexAttribute int32
var mapTimeUniform int32
var mapMouseUniform int32
var mapTextureUniform int32

func fromGLString(p *uint8) string {
	buffer := []byte{}

	for *p != 0 {
		buffer = append(buffer, *p)
		p = (*uint8)(unsafe.Pointer(uintptr(unsafe.Pointer(p)) + (uintptr(1))))
	}

	return string(buffer)
}

var lvl *Level

var ship ships.Ship

func generateMap() {
	hShift := (ship.Position.X() - float32(math.Floor(float64(ship.Position.X())))) * 16.0

	// Fill vbo.
	data := []float32{}
	leftMostTile := uint(ship.Position.X()) - (config.Screen.ResX/16)/2
	rightMostTile := uint(ship.Position.X()) + (config.Screen.ResX/16)/2
	topMostTile := uint(ship.Position.Y()) - (config.Screen.ResY/16)/2
	bottomMostTile := uint(ship.Position.Y()) + (config.Screen.ResY/16)/2
	for y := topMostTile; y < bottomMostTile; y++ {
		for x := leftMostTile; x < rightMostTile+1; x++ {
			tile := lvl.Tile(x, y)
			if tile == 0 {
				continue
			}

			tx, ty := lvl.Texel(tile)

			data = append(data,
				float32((x-leftMostTile)*16)+hShift, float32((y-topMostTile)*16),
				tx, ty,

				float32((x+1-leftMostTile)*16)+hShift, float32((y-topMostTile)*16),
				tx+16.0/306.0, ty,

				float32((x+1-leftMostTile)*16)+hShift, float32((y+1-topMostTile)*16),
				tx+16.0/306.0, ty+16.0/160.0,
			)

			data = append(data,
				float32((x+1-leftMostTile)*16)+hShift, float32((y+1-topMostTile)*16),
				tx+16.0/306.0, ty+16.0/160.0,

				float32((x-leftMostTile)*16)+hShift, float32((y+1-topMostTile)*16),
				tx, ty+16.0/160.0,

				float32((x-leftMostTile)*16)+hShift, float32((y-topMostTile)*16),
				tx, ty,
			)

			//data = append(data, -1, -1, 1, -1, 1, 1, 1, 1, -1, 1, -1, -1)
		}
	}

	nMapVertecies = uint32(len(data))

	gl.BufferData(gl.ARRAY_BUFFER, len(data)*4, unsafe.Pointer(&data[0]), gl.DYNAMIC_DRAW)
}

func InitializeMap() {
	var err error
	lvl, err = NewLevelFromFile("dsbpub1.lvl")
	if err != nil {
		panic(err)
	}

	gl.GenVertexArrays(1, &mapVao)
	gl.BindVertexArray(mapVao)
	defer gl.BindVertexArray(0)

	gl.GenBuffers(1, &mapVbo)
	gl.BindBuffer(gl.ARRAY_BUFFER, mapVbo)

	//gl.VertexAttribPointer(1, 2, gl.FLOAT, false, 20, unsafe.Pointer(uintptr(8)))
	//gl.EnableVertexAttribArray(1)

	// Shaders.
	vs := gl.CreateShader(gl.VERTEX_SHADER)
	src, vsrcFree := gl.Strs(mapVertexShaderSrc)
	defer vsrcFree()
	gl.ShaderSource(vs, 1, src, nil)
	gl.CompileShader(vs)
	defer gl.DeleteShader(vs)
	var status int32
	var errorLog [512]uint8
	gl.GetShaderiv(vs, gl.COMPILE_STATUS, &status)
	gl.GetShaderInfoLog(vs, 512, nil, &errorLog[0])
	fmt.Println(fromGLString(&errorLog[0]))

	fs := gl.CreateShader(gl.FRAGMENT_SHADER)
	src, fssrcFree := gl.Strs(mapFragmentShaderSrc)
	defer fssrcFree()
	gl.ShaderSource(fs, 1, src, nil)
	gl.CompileShader(fs)
	defer gl.DeleteShader(fs)
	gl.GetShaderiv(fs, gl.COMPILE_STATUS, &status)
	gl.GetShaderInfoLog(fs, 512, nil, &errorLog[0])
	fmt.Println(fromGLString(&errorLog[0]))

	mapProgram = gl.CreateProgram()
	gl.AttachShader(mapProgram, vs)
	gl.AttachShader(mapProgram, fs)
	gl.LinkProgram(mapProgram)

	mapPosAttribute = gl.GetAttribLocation(mapProgram, gl.Str(string("a_pos\x00")))
	mapTexAttribute = gl.GetAttribLocation(mapProgram, gl.Str(string("a_tex\x00")))

	mapTimeUniform = gl.GetUniformLocation(mapProgram, gl.Str(string("time\x00")))
	mapMouseUniform = gl.GetUniformLocation(mapProgram, gl.Str(string("u_mouse\x00")))
	mapTextureUniform = gl.GetUniformLocation(mapProgram, gl.Str(string("u_mapTexture\x00")))

	gl.VertexAttribPointer(uint32(mapPosAttribute), 2, gl.FLOAT, false, 4*4, unsafe.Pointer(uintptr(0)))
	gl.EnableVertexAttribArray(uint32(mapPosAttribute))

	gl.VertexAttribPointer(uint32(mapTexAttribute), 2, gl.FLOAT, false, 4*4, unsafe.Pointer(uintptr(8)))
	gl.EnableVertexAttribArray(uint32(mapTexAttribute))

}

func RenderMap(start time.Time, mouseX, mouseY float64) {
	gl.UseProgram(mapProgram)
	runningTime := float64(time.Since(start).Seconds())
	_ = runningTime
	gl.Uniform1f(mapTimeUniform, float32(runningTime))

	gl.Uniform2f(mapMouseUniform, float32(mouseX), float32(mouseY))

	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, lvl.TextureID())
	gl.Uniform1i(mapTextureUniform, 0)

	gl.BindVertexArray(mapVao)
	gl.DrawArrays(gl.TRIANGLES, 0, int32(nMapVertecies))
}

func init() {
	runtime.LockOSThread()
}

func main() {
	/*go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()*/

	err := glfw.Init()
	if err != nil {
		panic(err)
	}
	defer glfw.Terminate()

	glfw.WindowHint(glfw.ClientAPI, glfw.OpenGLESAPI)

	glfw.WindowHint(glfw.ContextVersionMajor, 2)
	glfw.WindowHint(glfw.ContextVersionMinor, 0)

	window, err := glfw.CreateWindow(int(config.Screen.ResX), int(config.Screen.ResY), "Testing", nil, nil)
	if err != nil {
		panic(err)
	}

	window.MakeContextCurrent()

	if err := gl.Init(); err != nil {
		panic(err)
	}

	ship.Position[0] = 285.0
	ship.Position[1] = 265.0

	InitializeMap()

	cur := time.Now()
	start := time.Now()
	var diff time.Duration
	for !window.ShouldClose() {
		diff = time.Since(cur)
		cur = time.Now()
		dt := diff.Seconds()
		_ = dt
		gl.Clear(gl.COLOR_BUFFER_BIT)

		mouseX, mouseY := window.GetCursorPos()

		if window.GetKey(glfw.KeyD) == glfw.Press {
			ship.Position[0] += float32(dt) * 16.0
		}

		if window.GetKey(glfw.KeyA) == glfw.Press {
			ship.Position[0] -= float32(dt) * 16.0
		}

		if window.GetKey(glfw.KeyW) == glfw.Press {
			ship.Position[1] -= float32(dt) * 16.0
		}

		if window.GetKey(glfw.KeyS) == glfw.Press {
			ship.Position[1] += float32(dt) * 16.0
		}

		generateMap()
		RenderMap(start, mouseX, mouseY)

		// Do OpenGL stuff.
		window.SwapBuffers()
		glfw.PollEvents()
	}
}
