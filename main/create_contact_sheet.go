package main

import (
	"bytes"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/math/fixed"
	"image"
	"image/jpeg"
	"io/ioutil"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/golang/freetype"
	"github.com/jung-kurt/gofpdf"
	"github.com/schollz/progressbar/v3"
)

type ContactSheetItem struct {
	Width            int
	Height           int
	HorizontalMargin int
	VerticalMargin   int
}

func main() {

	start := time.Now()
	programPath, err := os.Getwd()
	//TODO: convert to program arguments (i.e. filename, columns, image resolution)
	const contactSheetFilename = "contactsheet.pdf"
	const columns = 5

	var validExt = [...]string{
		".jpeg",
		".jpg",
	}

	if err != nil {
		log.Fatal(err)
	}

	fileNames, err := ioutil.ReadDir(programPath)

	if err != nil {
		log.Fatal(err)
	}

	destFiles := make([]string, 0)

	for _, file := range fileNames {
		fileName := file.Name()
		extension := filepath.Ext(fileName)

		// only process files with valid image extensions
		if len(extension) > 0 && inSortedList(extension, validExt[:]) {
			destFiles = append(destFiles, fileName)
		}
	}

	log.Printf("Creating Contact Sheet...")

	err = generateContactSheet(destFiles, filepath.Join(programPath, contactSheetFilename), columns)
	if err != nil {
		log.Fatal(err)
	}

	duration := time.Since(start)
	log.Printf("Completed Process in %f seconds.", duration.Seconds())
}

func generateContactSheet(fileNames []string, outputFile string, columns int) error {
	pageSize := 11 / 8.5
	rows := int(math.Floor(float64(columns) * pageSize))
	contactSheetItem := ContactSheetItem{Width: 600, Height: 600, HorizontalMargin: 20, VerticalMargin: 52}

	canvasWidth := (contactSheetItem.HorizontalMargin * (columns - 1)) + (columns * contactSheetItem.Width)
	canvasHeight := (contactSheetItem.VerticalMargin * (rows)) + (rows * contactSheetItem.Height)
	sp := image.Point{X: canvasWidth, Y: canvasHeight}

	contactSheetPdf := createPdf()

	fileIdx := 0
	lenFilenames := len(fileNames)
	var err error
	bar := progressbar.Default(int64(lenFilenames))

	for fileIdx >= 0 && fileIdx < lenFilenames {
		canvas := image.NewRGBA(image.Rectangle{Max: sp})
		draw.Draw(canvas, canvas.Bounds(), image.White, image.ZP, draw.Src)

		fileIdx, err = createImageGrid(bar, canvas, fileNames, rows, columns, fileIdx, contactSheetItem)
		if err != nil {
			return err
		}

		imageNum := int(math.Ceil(float64(fileIdx) / float64(rows*columns)))
		err = AddNextImage(contactSheetPdf, "image"+strconv.Itoa(imageNum), canvas)
		if err != nil {
			return err
		}
	}

	err = savePdf(contactSheetPdf, outputFile)
	if err != nil {
		return err
	}

	return nil
}

func createImageGrid(bar *progressbar.ProgressBar, canvas *image.RGBA, fileNames []string, rows int, columns int, fileIdx int, contactSheetItem ContactSheetItem) (int, error) {
	for row := 0; row < rows; row++ {
		for col := 0; col < columns; col++ {
			//last image processed within grid
			if fileIdx >= len(fileNames) {
				return -1, nil
			}
			fileName := fileNames[fileIdx]
			img, err := resizeImage(fileName, contactSheetItem.Height, contactSheetItem.Width)
			if err != nil {
				return 0, err
			}
			appendImage(canvas, img, contactSheetItem, row, col, fileName)
			fileIdx++
			_ = bar.Add(fileIdx)
		}
	}
	return fileIdx, nil
}

func inSortedList(value string, list []string) bool {
	i := sort.SearchStrings(list, value)
	return i < len(list) && list[i] == value
}

func resizeImage(filePath string, length int, width int) (*image.RGBA, error) {
	input, _ := os.Open(filePath)
	defer input.Close()

	src, err := jpeg.Decode(input)
	if err != nil {
		return nil, err
	}

	dst := image.NewRGBA(image.Rect(0, 0, width, length))

	//TODO: consider using a faster resizing algorithm that isn't native Go
	draw.BiLinear.Scale(dst, dst.Rect, src, src.Bounds(), draw.Over, nil)

	return dst, nil
}

func appendImage(canvas *image.RGBA, img *image.RGBA, contactSheetItem ContactSheetItem, j int, i int, caption string) {
	sp := image.Point{
		X: (i * contactSheetItem.Width) + (i * contactSheetItem.HorizontalMargin),
		Y: (j * contactSheetItem.Height) + (j * contactSheetItem.VerticalMargin),
	}

	r := image.Rectangle{Min: sp, Max: sp.Add(img.Bounds().Size())}
	draw.Draw(canvas, r, img, image.Point{}, draw.Src)
	addLabel(canvas, contactSheetItem, i, j, caption)
}

func addLabel(img *image.RGBA, contactSheetItem ContactSheetItem, x, y int, label string) {

	var (
		fgColor  image.Image
		fontFace *truetype.Font
		fontSize = 40.0
	)

	fgColor = image.Black
	fontFace, _ = freetype.ParseFont(goregular.TTF)

	d := &font.Drawer{
		Dst: img,
		Src: fgColor,
		Face: truetype.NewFace(fontFace, &truetype.Options{
			Size:    fontSize,
			Hinting: font.HintingFull,
		}),
	}

	labelLength := d.MeasureString(label)

	marginWidthOffSet := contactSheetItem.HorizontalMargin * x
	marginHeightOffSet := (float64(contactSheetItem.VerticalMargin))*float64(y) + (float64(contactSheetItem.VerticalMargin) / 1.3)

	d.Dot = fixed.Point26_6{
		X: fixed.I(contactSheetItem.Width*x) + (fixed.I(contactSheetItem.Width)-labelLength)/2 + fixed.I(marginWidthOffSet),
		Y: fixed.I(contactSheetItem.Height*(y+1) + int(marginHeightOffSet)),
	}

	d.DrawString(label)
}

func createPdf() *gofpdf.Fpdf {
	fdfp := gofpdf.New("P", "mm", "A4", "")
	fdfp.SetMargins(0, 0, 0)
	return fdfp
}

func AddNextImage(pdf *gofpdf.Fpdf, imageName string, img *image.RGBA) error {

	const imageType = "JPG"

	pdf.AddPage()
	buf := new(bytes.Buffer)

	err := jpeg.Encode(buf, img, &jpeg.Options{
		Quality: 95,
	})
	if err != nil {
		return err
	}

	// convert buffer to reader
	reader := bytes.NewReader(buf.Bytes())
	pdf.RegisterImageOptionsReader(imageName, gofpdf.ImageOptions{ImageType: imageType, ReadDpi: true}, reader)
	pdf.ImageOptions(imageName, 2, 2, 206, 0, false, gofpdf.ImageOptions{ImageType: imageType, ReadDpi: true}, 0, "")

	return nil
}

func savePdf(pdf *gofpdf.Fpdf, filename string) error {
	return pdf.OutputFileAndClose(filename)
}
