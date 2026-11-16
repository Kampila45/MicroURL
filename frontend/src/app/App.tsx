import { BrowserRouter, Route, Routes } from "react-router-dom";
import { StatsPage } from "../features/analytics/StatsPage";
import { ExpiredPage } from "../features/errors/ExpiredPage";
import { NotFoundPage } from "../features/errors/NotFoundPage";
import { ServerErrorPage } from "../features/errors/ServerErrorPage";
import { useShortenStore } from "../features/shorten";
import { HomePage } from "../features/shorten/components/HomePage";
import { Footer } from "../shared/components/Footer";
import { TopNavBar } from "../shared/components/TopNavBar";

export function App() {
  const resetStore = useShortenStore((state) => state.reset);

  return (
    <BrowserRouter>
      <div className="min-h-screen flex flex-col justify-between bg-background text-on-surface">
        <TopNavBar onResetHome={resetStore} />
        <Routes>
          <Route path="/" element={<HomePage />} />
          <Route path="/stats" element={<StatsPage />} />
          <Route path="/stats/:slug" element={<StatsPage />} />
          <Route path="/404" element={<NotFoundPage />} />
          <Route path="/410" element={<ExpiredPage />} />
          <Route path="/500" element={<ServerErrorPage />} />
          <Route path="*" element={<NotFoundPage />} />
        </Routes>
        <Footer />
      </div>
    </BrowserRouter>
  );
}
