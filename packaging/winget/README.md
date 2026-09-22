# WinGet staging

After the first GitHub Release is verified, generate a versioned WinGet manifest with:

- package identifier `WhichRepo.WhichRepo`;
- publisher `WhichRepo`;
- installer type `zip` with `whichrepo.exe` as the nested portable file;
- amd64 and arm64 release URLs;
- SHA-256 values from `checksums.txt`;
- MIT license and the GitHub repository URL.

Validate it with `winget validate` before opening a `microsoft/winget-pkgs` pull request. Publication is intentionally deferred until the package identifier and release assets exist.

