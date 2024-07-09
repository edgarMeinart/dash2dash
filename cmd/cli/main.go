package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"sync"

	"net/http"

	"github.com/abema/go-mp4"
	"github.com/gin-gonic/gin"
	"github.com/sunfish-shogi/bufseekio"
	ffmpeg "github.com/u2takey/ffmpeg-go"
)

func urlOpen(url string) (io.Reader, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("bad status: %s", resp.Status)
	}

	return resp.Body, nil
}

func downloadFile(url string, wg *sync.WaitGroup, ch chan<- io.Reader) {
	defer wg.Done()
	file, err := urlOpen(url)
	if err != nil {
		log.Printf("Failed to open URL: %v", err)
		ch <- nil
		return
	}
	ch <- file
}

func watermarkSegment(m4sChunkFileUrl string, responseWriter http.ResponseWriter) {
	mp4InitFileUrl := "http://localhost:8000/init-0.mp4"
	// m4sChunkFileUrl := "http://localhost:8000/chunk-0-00003-original.m4s"
	// outputPath := "testdata/example_1/chunk-0-00003.m4s"
	watermarkText := m4sChunkFileUrl

	//
	// DOWNLOAD SEGMENTS FILES
	//
	var wg sync.WaitGroup
	ch1 := make(chan io.Reader, 1)
	ch2 := make(chan io.Reader, 1)

	wg.Add(2)
	go downloadFile(mp4InitFileUrl, &wg, ch1)
	go downloadFile(m4sChunkFileUrl, &wg, ch2)

	wg.Wait()
	close(ch1)
	close(ch2)

	var mp4InitFile, m4sSegmentReader io.Reader

	for {
		select {
		case file, ok := <-ch1:
			log.Println("Received ch1")
			if !ok {
				ch1 = nil
				continue
			}
			if mp4InitFile == nil {
				mp4InitFile = file
			}
		case file, ok := <-ch2:
			log.Println("Received ch2")
			if !ok {
				ch2 = nil
				continue
			}
			if m4sSegmentReader == nil {
				m4sSegmentReader = file
			}
		}

		if m4sSegmentReader != nil && mp4InitFile != nil {
			log.Println("Received ch1 and ch2: done")
			break
		}
	}

	defer mp4InitFile.(io.Closer).Close()
	defer m4sSegmentReader.(io.Closer).Close()

	segmentTmpFile, err := os.CreateTemp("", "segment.mp4")
	if err != nil {
		log.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(segmentTmpFile.Name())
	defer segmentTmpFile.Close()

	if _, err := io.Copy(segmentTmpFile, m4sSegmentReader); err != nil {
		log.Fatalf("Failed to copy data: %v", err)
	}

	//
	// GET INFO FROM CURRENT SEGMENT SIDX ATOM
	//
	segmentTmpFile.Seek(0, io.SeekStart)
	sidxBoxes, err := mp4.ExtractBoxWithPayload(segmentTmpFile, nil, mp4.BoxPath{mp4.BoxTypeSidx()})
	if err != nil {
		panic(err)
	}

	if len(sidxBoxes) == 0 {
		log.Println(segmentTmpFile.Name())
		panic(errors.New("SIDX atom not found"))
	}

	sidx := sidxBoxes[0].Payload.(*mp4.Sidx)
	segmentTimescale := sidx.Timescale
	segmentSize := sidx.References[0].ReferencedSize
	segmentDuration := sidx.References[0].SubsegmentDuration
	earliestPresentationTime := sidx.GetEarliestPresentationTime()

	//
	// GET SEGMENT INFO FROM MOOF->MFHD
	//

	mfhdBoxes, err := mp4.ExtractBoxWithPayload(segmentTmpFile, nil, mp4.BoxPath{mp4.BoxTypeMoof(), mp4.BoxTypeMfhd()})
	if err != nil {
		panic(err)
	}

	if len(mfhdBoxes) == 0 {
		panic(errors.New("MOOF->MFHD atom not found"))
	}
	currentSegmentSequenceNumber := mfhdBoxes[0].Payload.(*mp4.Mfhd).SequenceNumber

	//
	// GET SEGMENT INFO FROM MOOF->TRAF->TFDT
	//

	tfdtBoxes, err := mp4.ExtractBoxWithPayload(segmentTmpFile, nil, mp4.BoxPath{mp4.BoxTypeMoof(), mp4.BoxTypeTraf(), mp4.BoxTypeTfdt()})
	if err != nil {
		panic(err)
	}

	if len(tfdtBoxes) == 0 {
		panic(errors.New("MOOF->TRAF->TFDT atom not found"))
	}

	currentSegmentBaseMediaDecodeTime := tfdtBoxes[0].Payload.(*mp4.Tfdt).GetBaseMediaDecodeTime()

	//
	// WATERMARK PART
	//
	combinedTmpFile, err := os.CreateTemp("", "combined.mp4")
	if err != nil {
		panic(err)
	}
	defer os.Remove(combinedTmpFile.Name())
	defer combinedTmpFile.Close()

	if _, err := io.Copy(combinedTmpFile, mp4InitFile); err != nil {
		panic(err)
	}

	segmentTmpFile.Seek(0, io.SeekStart)
	if _, err := io.Copy(combinedTmpFile, segmentTmpFile); err != nil {
		panic(err)
	}

	combinedWatermarkedTmpFile, err := os.CreateTemp("", "combined_watermarked.mp4")
	if err != nil {
		panic(err)
	}
	defer os.Remove(combinedWatermarkedTmpFile.Name())
	defer combinedWatermarkedTmpFile.Close()

	//
	// DRAW WATERMARK USING FFMPEG
	//
	ffmpegCombiner := ffmpeg.Input(combinedTmpFile.Name(), ffmpeg.KwArgs{})

	ffmpegCombiner = ffmpegCombiner.Filter("drawtext", ffmpeg.Args{}, ffmpeg.KwArgs{
		"text":         watermarkText,
		"fontcolor":    "white",
		"fontsize":     "24",
		"box":          "1",
		"boxcolor":     "black@0.5",
		"boxborderw":   "5",
		"x":            "(w-text_w)/2",
		"y":            "(h-text_h)/2",
	})

	outputKw := ffmpeg.KwArgs{
		"c:a": "copy",
		"f": "mp4",
		"movflags": "empty_moov+default_base_moof",
		"frag_type": "duration",
		"frag_duration": "10000000", // the max possible duration is set to prevent multiple fragments generation in one segment
	}

	ffmpegCombiner = ffmpegCombiner.ErrorToStdOut().Output(combinedWatermarkedTmpFile.Name(), outputKw).OverWriteOutput()

	err = ffmpegCombiner.Run()
	if err != nil {
		panic(err)
	}

	//
	// END DRAW WATERMARK
	//

	inputFile, err := os.Open(combinedWatermarkedTmpFile.Name())
	if err != nil {
		log.Panic(err)
	}
	defer inputFile.Close()

	outputTmpFile, err := os.CreateTemp("", "output.mp4")
	// outputFile, err := os.Create(outputPath)
	if err != nil {
		log.Panic(err)
	}
	defer os.Remove(outputTmpFile.Name())
	defer outputTmpFile.Close()

	r := bufseekio.NewReadSeeker(inputFile, 128*1024, 4)
	w := mp4.NewWriter(outputTmpFile)

	_, err = mp4.ReadBoxStructure(r, func(h *mp4.ReadHandle) (interface{}, error) {
		if h.BoxInfo.Type == mp4.BoxTypeMoov() || h.BoxInfo.Type == mp4.BoxTypeMfra() {
			return uint64(0), nil
		}

		if h.BoxInfo.Type == mp4.BoxTypeFtyp() {
			// ADD STYP ATOM
			w.StartBox(&mp4.BoxInfo{Type: mp4.BoxTypeStyp()})

			msdh := mp4.CompatibleBrandElem{CompatibleBrand: [4]byte{'m', 's', 'd', 'h'} }
			msix := mp4.CompatibleBrandElem{CompatibleBrand: [4]byte{'m', 's', 'i', 'x'} }
			compatibleBrands := []mp4.CompatibleBrandElem{msdh, msix}

			styp := &mp4.Styp{
				MajorBrand: [4]byte{'m', 's', 'd', 'h'},
				MinorVersion: 0,
				CompatibleBrands: compatibleBrands,
			}
			ctx := h.BoxInfo.Context

			if _, err := mp4.Marshal(w, styp, ctx); err != nil {
				return nil, err
			}

			if _, err := w.EndBox(); err != nil {
				return nil, err
			}
			// END STYP

			// ADD SIDX
			w.StartBox(&mp4.BoxInfo{Type: mp4.BoxTypeSidx()})

			sidxReferences := []mp4.SidxReference{
				{
					ReferenceType: false,
					ReferencedSize: segmentSize, // FIXME
					SubsegmentDuration: segmentDuration, // FIXME
					StartsWithSAP: true,
					SAPType: 0,
					SAPDeltaTime: 0,
				},
			}
			sidx := &mp4.Sidx{
				ReferenceID: 1,
				Timescale: segmentTimescale,
				EarliestPresentationTimeV0: uint32(earliestPresentationTime),
				FirstOffsetV0: 0,
				ReferenceCount: 1,
				References: sidxReferences,
			}
			ctx = h.BoxInfo.Context
			if _, err := mp4.Marshal(w, sidx, ctx); err != nil {
				return nil, err
			}

			if _, err := w.EndBox(); err != nil {
				return nil, err
			}
			// END SIDX

			return uint64(0), nil
		}

		if !h.BoxInfo.IsSupportedType() || h.BoxInfo.Type == mp4.BoxTypeMdat() {
			return nil, w.CopyBox(r, &h.BoxInfo)
		}

		// write header
		_, err := w.StartBox(&h.BoxInfo)
		if err != nil {
			return nil, err
		}
		// read payload
		box, _, err := h.ReadPayload()
		if err != nil {
			return nil, err
		}

		if h.BoxInfo.Type == mp4.BoxTypeMfhd() {
			mfhd := box.(*mp4.Mfhd)
			mfhd.SequenceNumber = currentSegmentSequenceNumber
		}

		if h.BoxInfo.Type == mp4.BoxTypeTfdt() {
			tfdt := box.(*mp4.Tfdt)
			if tfdt.GetVersion() == 0 {
				tfdt.BaseMediaDecodeTimeV0 = uint32(currentSegmentBaseMediaDecodeTime)
			} else {
				tfdt.BaseMediaDecodeTimeV1 = uint64(currentSegmentBaseMediaDecodeTime)
			}
		}

		// write payload
		if _, err := mp4.Marshal(w, box, h.BoxInfo.Context); err != nil {
			return nil, err
		}
		// expand all of offsprings
		if _, err := h.Expand(); err != nil {
			return nil, err
		}

		// rewrite box size
		_, err = w.EndBox()

		return nil, err
	})

	outputTmpFile.Seek(0, io.SeekStart)
	_, err = io.Copy(responseWriter, outputTmpFile)

	if err != nil {
		log.Fatal(fmt.Sprintf("Failed to write file to response: %v", err))
		http.Error(responseWriter, fmt.Sprintf("Failed to write file to response: %v", err), http.StatusInternalServerError)
		return
	}
}

func main() {
	router := gin.Default()

	router.GET("/video/:endpoint_name", func(c *gin.Context) {
			endpointName := c.Param("endpoint_name")
			readSegmentURL := fmt.Sprintf("http://localhost:8000/%s", endpointName)

			// Call watermarkSegment function (or any other processing logic)
			watermarkSegment(readSegmentURL, c.Writer)

			// Optionally, you can send a response to the client
			// c.String(http.StatusOK, "You have hit the endpoint: %s", endpointName)
		})

		// Run the server on port 8080
		if err := router.Run(":8080"); err != nil {
			panic(err)
		}
}
