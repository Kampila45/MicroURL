export function Footer() {
  const currentYear = new Date().getFullYear();

  return (
    <footer className="w-full bg-background/90 border-t border-outline-variant mt-auto">
      <div className="w-full max-w-[820px] mx-auto px-4 sm:px-8 py-4 flex justify-center items-center text-xs text-outline text-center">
        <span>© {currentYear} MicroURL</span>
      </div>
    </footer>
  );
}
