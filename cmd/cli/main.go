package main

import (
	"log"
	"os"

	"github.com/abema/go-mp4"
	"github.com/sunfish-shogi/bufseekio"
)


func main() {
	inputFile, err := os.Open("tmp/output/combined-2.mp4")
	if err != nil {
		log.Panic(err)
	}
	defer inputFile.Close()

	outputFile, err := os.Create("tmp/output/combined-3.m4s")
	if err != nil {
		log.Panic(err)
	}
	defer outputFile.Close()

	r := bufseekio.NewReadSeeker(inputFile, 128*1024, 4)
	w := mp4.NewWriter(outputFile)

	_, err = mp4.ReadBoxStructure(r, func(h *mp4.ReadHandle) (interface{}, error) {
		if h.BoxInfo.Type == mp4.BoxTypeMoov() || h.BoxInfo.Type == mp4.BoxTypeMfra() {
			return uint64(0), nil
		}

		if h.BoxInfo.Type == mp4.BoxTypeFtyp() {
			// ADD STYP
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
				mp4.SidxReference{
					ReferenceType: false,
					ReferencedSize: 3040256, // FIXME
					SubsegmentDuration: 216216, // FIXME
					StartsWithSAP: true,
					SAPType: 0,
					SAPDeltaTime: 0,
				},
			}
			sidx := &mp4.Sidx{
				ReferenceID: 1,
				Timescale: 24000,
				EarliestPresentationTimeV0: 0,
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

}
