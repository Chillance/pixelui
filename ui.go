package pixelui

import (
	"C"

	"github.com/faiface/pixel"
	"github.com/inkyblackness/imgui-go"
)
import (
	"image/color"
	"time"
	"unsafe"

	"github.com/faiface/mainthread"
	"github.com/faiface/pixel/pixelgl"
)

const uiShader = `
#version 330 core
in vec4  vColor;
in vec2  vTexCoords;
in float vIntensity;

out vec4 fragColor;

uniform vec4 uColorMask;
uniform vec4 uTexBounds;
uniform sampler2D uTexture;
uniform vec4 uClipRect;

void main() {
	fragColor *= vColor * texture(uTexture, vTexCoords).a;
}
`

// UI Stores the state of the pixelui UI
type UI struct {
	win     *pixelgl.Window
	context *imgui.Context
	io      imgui.IO
	fonts   imgui.FontAtlas
	timer   time.Time
	pic     *pixel.PictureData
	picture pixel.TargetPicture
	shader  *pixelgl.GLShader
	matrix  pixel.Matrix
}

// NewUI Creates the UI and setups up its internal structures
func NewUI(win *pixelgl.Window) *UI {
	var context *imgui.Context
	mainthread.Call(func() {
		context = imgui.CreateContext(nil)
	})

	ui := &UI{
		win:     win,
		context: context,
	}

	ui.matrix = pixel.IM.ScaledXY(win.Bounds().Center(), pixel.V(1, -1))

	ui.io = imgui.CurrentIO()
	ui.io.SetDisplaySize(pixelVecToimguiVec(win.Bounds().Size()))

	ui.fonts = ui.io.Fonts()
	ui.fonts.AddFontDefault()
	f := ui.fonts.TextureDataAlpha8()

	ui.pic = pixel.MakePictureData(pixel.R(0, 0, float64(f.Width), float64(f.Height)))

	for y := 0; y < f.Height; y++ {
		for x := 0; x < f.Width; x++ {
			i := y*f.Width + x
			ptr := (*uint8)(unsafe.Pointer(uintptr(f.Pixels) + uintptr(i)))
			ui.pic.Pix[i] = color.RGBA{R: 0, G: 0, B: 0, A: *ptr}
		}
	}

	ui.shader = pixelgl.NewGLShader(uiShader)

	ui.picture = win.Canvas().MakePicture(ui.pic)
	ui.setKeyMapping()

	return ui
}

// Destroy cleans up the imgui context
func (ui *UI) Destroy() {
	ui.context.Destroy()
}

// NewFrame Call this at the beginning of the frame to tell the UI that the frame has started
func (ui *UI) NewFrame() {
	ui.timer = time.Now()

	// imgui requires that io be set before calling NewFrame
	ui.prepareIO()

	imgui.NewFrame()
}

// update Handles general update type things and handle inputs. Called from ui.Draw.
func (ui *UI) update() {
	ui.io.SetDeltaTime(float32(time.Since(ui.timer).Seconds()))
}

// Draw Draws the imgui UI to the Pixel Window
func (ui *UI) Draw(win *pixelgl.Window) {
	win.SetComposeMethod(pixel.ComposeOver)

	// imgui draws things from top-left as 0,0 where Pixel draws from bottom-left as 0,0,
	//	for drawing and handling inputs, we need to "flip" imgui.
	ui.update()

	// Tell imgui to render and get the resulting draw data
	imgui.Render()
	data := imgui.RenderedDrawData()

	// In each command, there is a vertex buffer that holds all of the vertices to draw;
	// 	there's also an index buffer which stores the indices into the vertex buffer that should
	//	be draw together. The vertex buffer is shared between multiple commands.
	vertexSize, posOffset, uvOffset, colOffset := imgui.VertexBufferLayout()
	indexSize := imgui.IndexBufferLayout()
	for _, cmds := range data.CommandLists() {
		var indexBufferOffset uintptr
		start, _ := cmds.VertexBuffer()
		idxStart, _ := cmds.IndexBuffer()

		for _, cmd := range cmds.Commands() {
			if cmd.HasUserCallback() {
				cmd.CallUserCallback(cmds)
			} else {
				win.SetMatrix(ui.matrix)
				tris := pixel.MakeTrianglesData(cmd.ElementCount())

				for i := 0; i < cmd.ElementCount(); i++ {
					idx := unsafe.Pointer(uintptr(idxStart) + indexBufferOffset)
					index := *(*uint16)(idx)
					ptr := unsafe.Pointer(uintptr(start) + (uintptr(int(index) * vertexSize)))
					pos := (*imgui.Vec2)(unsafe.Pointer(uintptr(ptr) + uintptr(posOffset)))
					uv := (*imgui.Vec2)(unsafe.Pointer(uintptr(ptr) + uintptr(uvOffset)))
					col := (*uint32)(unsafe.Pointer(uintptr(ptr) + uintptr(colOffset)))

					position := imguiVecToPixelVec(*pos)
					color := imguiColorToPixelColor(*col)
					uuvv := imguiVecToPixelVec(*uv)

					(*tris)[i].Position = position
					(*tris)[i].Picture = uuvv
					(*tris)[i].Color = pixel.ToRGBA(color)
					(*tris)[i].Intensity = 0
					indexBufferOffset += uintptr(indexSize)
				}

				clipRect := imguiRectToPixelRect(cmd.ClipRect())
				clipRect.Min = ui.matrix.Project(clipRect.Min)
				clipRect.Max = ui.matrix.Project(clipRect.Max)
				shaderTris := pixelgl.NewGLTriangles(ui.shader, tris)
				shaderTris.SetClipRect(clipRect)
				ui.picture.Draw(win.Canvas().MakeTriangles(shaderTris))
			}
		}
	}

	win.SetMatrix(pixel.IM)
}

// imguiColorToPixelColor Converts the imgui color to a Pixel color
func imguiColorToPixelColor(c uint32) color.RGBA {
	// ABGR -> RGBA
	return color.RGBA{
		A: uint8((c >> 24) & 0xFF),
		B: uint8((c >> 16) & 0xFF),
		G: uint8((c >> 8) & 0xFF),
		R: uint8(c & 0xFF),
	}
}

// imguiVecToPixelVec Converts the imgui vector to a Pixel vector
func imguiVecToPixelVec(v imgui.Vec2) pixel.Vec {
	return pixel.V(float64(v.X), float64(v.Y))
}

// imguiRectToPixelRect Converts the imgui rect to a Pixel rect
func imguiRectToPixelRect(r imgui.Vec4) pixel.Rect {
	return pixel.R(float64(r.X), float64(r.Y), float64(r.Z), float64(r.W))
}

func pixelVecToimguiVec(v pixel.Vec) imgui.Vec2 {
	return imgui.Vec2{X: float32(v.X), Y: float32(v.Y)}
}
