This fork add support for extracting multipart zips like `.z01`, `.z02`, ..., `.zip`.

The work is based on <https://github.com/alexmullins/zip>

The work related to Standard Zip Encryption is based on <https://github.com/yeka/zip>

## Usage

- Call `OpenReader` and pass the filename of the volume with the `.zip` suffix to open the entire multipart archive. (Totally same to `alexmullins/zip`)

- Call `NewMultipartReader` and provide each volume as a `SizeReaderAt`. The volume with the `.zip` suffix must be the last one in the sequence.

- Defination of `SizeReaderAt`:
  ```go
  type SizeReaderAt interface {
      Size() int64
      io.ReaderAt
  }
  ```
