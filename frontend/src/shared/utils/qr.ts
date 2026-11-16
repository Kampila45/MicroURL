import QRCode from "qrcode";

export async function generateQRCodeDataURL(url: string): Promise<string> {
  return QRCode.toDataURL(url, {
    errorCorrectionLevel: "M",
    type: "image/png",
    margin: 2,
    width: 256,
    color: {
      dark: "#F7F8F8",
      light: "#0B0C0E",
    },
  });
}

export async function generateQRCodeSVG(url: string): Promise<string> {
  return QRCode.toString(url, {
    errorCorrectionLevel: "M",
    type: "svg",
    margin: 2,
    width: 256,
    color: {
      dark: "#0B0C0E",
      light: "#FFFFFF",
    },
  });
}

export function triggerFileDownload(href: string, filename: string): void {
  const link = document.createElement("a");
  link.href = href;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
}
