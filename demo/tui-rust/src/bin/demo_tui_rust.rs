use std::sync::Arc;

use demo_tui_rust::tui::adapter::crossterm_keyboard::{CrosstermKeyboard, CrosstermKeyboardConfig};
use demo_tui_rust::tui::adapter::crossterm_screen::{CrosstermScreen, CrosstermScreenConfig};
use demo_tui_rust::tui::controller::{BoardController, BoardControllerImpl};
use demo_tui_rust::tui::driver::tui_driver::{
    error_chain, TuiDriver, TuiDriverConfig, TuiDriverError,
};

#[tokio::main]
async fn main() {
    if let Err(error) = run().await {
        eprintln!("demo-tui-rust: {}", error_chain(&error));
        std::process::exit(1);
    }
}

async fn run() -> Result<(), TuiDriverError> {
    let controller: Arc<dyn BoardController + Send + Sync> = Arc::new(BoardControllerImpl::new(
        Arc::new(CrosstermScreen::new(CrosstermScreenConfig::default())),
        Arc::new(CrosstermKeyboard::new(CrosstermKeyboardConfig::default())),
    ));

    let mut driver = TuiDriver::new(TuiDriverConfig::default(), controller);

    driver.bind().await?;
    driver.announce()?;
    driver.serve().await
}
