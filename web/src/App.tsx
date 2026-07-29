import { BrowserRouter, Route, Routes } from 'react-router'
import AppLayout from './components/AppLayout'
import HomePage from './pages/HomePage'
import ArtistsPage from './pages/ArtistsPage'
import ArtistDetailPage from './pages/ArtistDetailPage'
import AlbumsPage from './pages/AlbumsPage'
import AlbumDetailPage from './pages/AlbumDetailPage'
import SongsPage from './pages/SongsPage'
import ImportPage from './pages/ImportPage'
import LibrariesPage from './pages/LibrariesPage'
import AboutPage from './pages/AboutPage'
import SettingsPage from './pages/SettingsPage'

function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route element={<AppLayout />}>
          <Route index element={<HomePage />} />
          <Route path="artists" element={<ArtistsPage />} />
          <Route path="artists/:id" element={<ArtistDetailPage />} />
          <Route path="albums" element={<AlbumsPage />} />
          <Route path="albums/:id" element={<AlbumDetailPage />} />
          <Route path="songs" element={<SongsPage />} />
          <Route path="import" element={<ImportPage />} />
          <Route path="libraries" element={<LibrariesPage />} />
          <Route path="settings" element={<SettingsPage />} />
          <Route path="about" element={<AboutPage />} />
        </Route>
      </Routes>
    </BrowserRouter>
  )
}

export default App
