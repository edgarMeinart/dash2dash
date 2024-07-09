### dash2dash

#### Description

This repository serves as a proof of concept for real-time watermarking technology. It includes a simple pseudo-DASH server that resolves a local DASH manifest. A proxy server intercepts video segments and overlays text using ffmpeg to demonstrate the watermarking function.

#### Requirements

- **go1.22.1**: Required for the watermarking proxy server written in Go. It may work with other versions of Go.
- **ruby 3.3.2**: Needed to run the pseudo-DASH server. In reality, any Ruby version should suffice as it runs a basic WEBrick server.
- **foreman**: Used to execute the Procfile, which in turn starts the demonstration.
- **abema/go-mp4**: Go library for parsing MP4 files.
- **u2takey/ffmpeg-go**: Go bindings for ffmpeg.
- **ffmpeg (version 6.1.1)**: Installed separately, used by the watermarking proxy server to overlay text on video segments.

#### Usage

To run the demonstration:

1. **Setup**:
   - Clone the repository.
   - Install necessary dependencies.

2. **Download DASH Segments**:
   - Download the DASH segments from [this Dropbox link](https://www.dropbox.com/scl/fo/2m851ywsc006numapq86y/AIk1kZNbfxPNcn4tl8ffmY0?rlkey=b1unr8sy0ww6fs7ug5so2atnm&st=wmf0wwe2&dl=0).

3. **Extract and Place Files**:
   - Unzip the downloaded file.
   - Place the extracted files into the project directory under `testdata/example_1/`. For example, the files should be structured as follows:
     ```
     <repo_dir>/testdata/example_1/chunk-0-00001.m4s
     <repo_dir>/testdata/example_1/chunk-0-00002.m4s
     ...
     ```

4. **Start Servers**:
   - Run the pseudo-DASH server:
     ```bash
     ruby bin/simple_http_server.rb
     ```
   - Start the watermarking proxy server:
     ```bash
     go run cmd/cli/main.go
     ```

5. **OR Run the Demonstration**:
   - Execute Foreman to start both servers simultaneously:
     ```bash
     foreman start
     ```

6. **Access the Demo**:
   - Open a browser or media player that supports DASH streaming and navigate to the demo URL (http://localhost:8000/manifest.mpd)

#### Contribution

Contributions are welcome! If you have suggestions or improvements, feel free to fork the repository and submit a pull request.

#### License

This project is licensed under the MIT License.
