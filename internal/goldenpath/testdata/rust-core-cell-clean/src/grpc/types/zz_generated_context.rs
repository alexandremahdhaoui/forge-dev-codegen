pub struct Context {
    pub session_id: [u8; 16],
    pub peer: std::net::SocketAddr,
}
