class KodeStream < Formula
  desc "Local-first planning and docs workflow tool"
  homepage "https://github.com/kriskhoavu/kode-stream"
  version "1.0.0"

  if OS.mac? && Hardware::CPU.arm?
    url "https://github.com/kriskhoavu/kode-stream/releases/download/v#{version}/kode-stream_#{version}_darwin_arm64.tar.gz"
    sha256 "REPLACE_DARWIN_ARM64_SHA256"
  elsif OS.mac? && Hardware::CPU.intel?
    url "https://github.com/kriskhoavu/kode-stream/releases/download/v#{version}/kode-stream_#{version}_darwin_amd64.tar.gz"
    sha256 "REPLACE_DARWIN_AMD64_SHA256"
  else
    odie "kode-stream Homebrew formula currently supports macOS only"
  end

  def install
    bin.install "kode-stream"
  end

  test do
    output = shell_output("#{bin}/kode-stream 2>&1", 2)
    assert_match "Usage", output
  end
end
