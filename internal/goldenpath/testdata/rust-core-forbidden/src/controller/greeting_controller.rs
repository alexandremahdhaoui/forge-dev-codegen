pub fn read_config() -> String {
    std::fs::read_to_string("config.toml").unwrap()
}
