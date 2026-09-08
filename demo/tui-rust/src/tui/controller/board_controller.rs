use crate::tui::controller::{BoardController, BoardControllerError, BoardControllerImpl};
use crate::tui::types::frame::{Frame, Step};
use crate::tui::types::key::Key;

const PLAYER: char = '@';

impl BoardController for BoardControllerImpl {
    fn on_key(&self, frame: &Frame, key: Key) -> Result<Step, BoardControllerError> {
        let (dx, dy) = match key {
            Key::Left => (-1, 0),
            Key::Right => (1, 0),
            Key::Up => (0, -1),
            Key::Down => (0, 1),
            Key::Act => return Ok(Step::Render(with_message(frame, "you act"))),
            Key::EndTurn => return Ok(Step::Render(with_message(frame, "the turn ends"))),
            Key::Quit => return Ok(Step::Quit),
        };

        Ok(Step::Render(moved(frame, dx, dy)))
    }

    fn on_line(&self, frame: &Frame, line: &str) -> Result<Step, BoardControllerError> {
        Ok(Step::Render(with_message(frame, line)))
    }

    fn on_tick(&self, frame: &Frame) -> Result<Frame, BoardControllerError> {
        if frame.grid.find(PLAYER).is_some() {
            return Ok(frame.clone());
        }

        let mut next = frame.clone();
        next.grid.set(0, 0, PLAYER);
        next.status = status_at(0, 0);

        Ok(next)
    }
}

fn with_message(frame: &Frame, message: &str) -> Frame {
    let mut next = frame.clone();
    next.message = message.to_string();
    next
}

fn moved(frame: &Frame, dx: i32, dy: i32) -> Frame {
    let mut next = frame.clone();
    let (x, y) = next.grid.find(PLAYER).unwrap_or((0, 0));
    let nx = stepped(x, dx, next.grid.width());
    let ny = stepped(y, dy, next.grid.height());

    next.grid.set(x, y, ' ');
    next.grid.set(nx, ny, PLAYER);
    next.status = status_at(nx, ny);

    next
}

fn stepped(at: u16, by: i32, size: u16) -> u16 {
    let last = (i32::from(size) - 1).max(0);
    let landed = (i32::from(at) + by).clamp(0, last);

    u16::try_from(landed).unwrap_or(0)
}

fn status_at(x: u16, y: u16) -> String {
    format!("@ at {x},{y}")
}

#[cfg(test)]
mod tests {
    use std::sync::{Arc, Mutex};

    use super::*;
    use crate::tui::driver::tui_driver::{
        TuiDriver, TuiDriverConfig, TuiDriverError, HEIGHT, WIDTH,
    };
    use crate::tui::port::keyboard::{KeyboardError, MockKeyboard};
    use crate::tui::port::screen::{MockScreen, ScreenError};
    use crate::tui::types::frame::Prompt;
    use crate::tui::types::key::Input;

    fn controller() -> BoardControllerImpl {
        BoardControllerImpl::new(Arc::new(MockScreen::new()), Arc::new(MockKeyboard::new()))
    }

    fn board_with_player_at(x: u16, y: u16) -> Frame {
        let mut frame = Frame::blank(WIDTH, HEIGHT);
        frame.grid.set(x, y, PLAYER);
        frame
    }

    fn rendered(step: Step) -> Frame {
        match step {
            Step::Render(frame) => frame,
            Step::Quit => panic!("the controller quit instead of rendering"),
        }
    }

    #[test]
    fn the_first_tick_places_the_at_sign_at_the_top_left() {
        let frame = controller()
            .on_tick(&Frame::blank(WIDTH, HEIGHT))
            .expect("a frame");

        assert_eq!(frame.grid.find(PLAYER), Some((0, 0)));
        assert_eq!(frame.status, "@ at 0,0");
    }

    #[test]
    fn a_later_tick_keeps_the_at_sign_where_it_is() {
        let frame = controller()
            .on_tick(&board_with_player_at(3, 2))
            .expect("a frame");

        assert_eq!(frame.grid.find(PLAYER), Some((3, 2)));
    }

    #[test]
    fn the_l_key_moves_the_at_sign_one_column_right() {
        let step = controller()
            .on_key(&board_with_player_at(3, 2), Key::Right)
            .expect("a step");

        let frame = rendered(step);

        assert_eq!(frame.grid.find(PLAYER), Some((4, 2)));
        assert_eq!(frame.grid.get(3, 2), Some(' '));
        assert_eq!(frame.status, "@ at 4,2");
    }

    #[test]
    fn the_h_j_and_k_keys_move_the_at_sign_left_down_and_up() {
        let start = board_with_player_at(3, 2);

        let left = rendered(controller().on_key(&start, Key::Left).expect("a step"));
        let down = rendered(controller().on_key(&start, Key::Down).expect("a step"));
        let up = rendered(controller().on_key(&start, Key::Up).expect("a step"));

        assert_eq!(left.grid.find(PLAYER), Some((2, 2)));
        assert_eq!(down.grid.find(PLAYER), Some((3, 3)));
        assert_eq!(up.grid.find(PLAYER), Some((3, 1)));
    }

    #[test]
    fn a_move_past_the_edge_keeps_the_at_sign_on_the_edge() {
        let top_left = board_with_player_at(0, 0);
        let bottom_right = board_with_player_at(WIDTH - 1, HEIGHT - 1);

        let left = rendered(controller().on_key(&top_left, Key::Left).expect("a step"));
        let up = rendered(controller().on_key(&top_left, Key::Up).expect("a step"));
        let right = rendered(
            controller()
                .on_key(&bottom_right, Key::Right)
                .expect("a step"),
        );
        let down = rendered(
            controller()
                .on_key(&bottom_right, Key::Down)
                .expect("a step"),
        );

        assert_eq!(left.grid.find(PLAYER), Some((0, 0)));
        assert_eq!(up.grid.find(PLAYER), Some((0, 0)));
        assert_eq!(right.grid.find(PLAYER), Some((WIDTH - 1, HEIGHT - 1)));
        assert_eq!(down.grid.find(PLAYER), Some((WIDTH - 1, HEIGHT - 1)));
    }

    #[test]
    fn the_q_key_quits() {
        let step = controller()
            .on_key(&board_with_player_at(0, 0), Key::Quit)
            .expect("a step");

        assert_eq!(step, Step::Quit);
    }

    #[test]
    fn the_act_and_end_turn_keys_write_the_message_line_and_leave_the_grid_alone() {
        let start = board_with_player_at(5, 1);

        let acted = rendered(controller().on_key(&start, Key::Act).expect("a step"));
        let ended = rendered(controller().on_key(&start, Key::EndTurn).expect("a step"));

        assert_eq!(acted.message, "you act");
        assert_eq!(ended.message, "the turn ends");
        assert_eq!(acted.grid, start.grid);
        assert_eq!(ended.grid, start.grid);
    }

    #[test]
    fn a_line_entered_with_enter_becomes_the_message_line() {
        let frame = rendered(
            controller()
                .on_line(&board_with_player_at(0, 0), "hello there")
                .expect("a step"),
        );

        assert_eq!(frame.message, "hello there");
    }

    struct Recording {
        drawn: Arc<Mutex<Vec<(Frame, Prompt)>>>,
        left: Arc<Mutex<u32>>,
    }

    fn recording_screen() -> (MockScreen, Recording) {
        let drawn: Arc<Mutex<Vec<(Frame, Prompt)>>> = Arc::new(Mutex::new(Vec::new()));
        let left = Arc::new(Mutex::new(0u32));

        let mut screen = MockScreen::new();
        screen.expect_enter().returning(|| Ok(()));

        let seen = drawn.clone();
        screen.expect_draw().returning(move |frame, prompt| {
            seen.lock()
                .expect("the draw log")
                .push((frame.clone(), prompt.clone()));
            Ok(())
        });

        let counted = left.clone();
        screen.expect_leave().returning(move || {
            *counted.lock().expect("the leave count") += 1;
            Ok(())
        });

        (screen, Recording { drawn, left })
    }

    fn scripted_keyboard(inputs: Vec<Option<Input>>) -> MockKeyboard {
        let mut remaining = inputs;
        remaining.reverse();

        let mut keyboard = MockKeyboard::new();
        keyboard
            .expect_read()
            .returning(move |_| Ok(remaining.pop().flatten()));

        keyboard
    }

    async fn play(screen: MockScreen, keyboard: MockKeyboard) -> Result<(), TuiDriverError> {
        let controller: Arc<dyn BoardController + Send + Sync> = Arc::new(
            BoardControllerImpl::new(Arc::new(screen), Arc::new(keyboard)),
        );

        let mut driver = TuiDriver::new(TuiDriverConfig::default(), controller);
        driver.bind().await?;
        driver.announce()?;
        driver.serve().await
    }

    #[tokio::test]
    async fn the_loop_draws_the_first_frame_moves_on_l_and_stops_on_q() {
        let (screen, recording) = recording_screen();
        let keyboard = scripted_keyboard(vec![Some(Input::Char('l')), Some(Input::Char('q'))]);

        play(screen, keyboard).await.expect("a clean exit");

        let drawn = recording.drawn.lock().expect("the draw log");

        assert_eq!(drawn.len(), 2);
        assert_eq!(drawn[0].0.grid.find(PLAYER), Some((0, 0)));
        assert_eq!(drawn[1].0.grid.find(PLAYER), Some((1, 0)));
        assert_eq!(*recording.left.lock().expect("the leave count"), 1);
    }

    #[tokio::test]
    async fn a_tick_with_no_key_redraws_through_the_controller() {
        let (screen, recording) = recording_screen();
        let keyboard = scripted_keyboard(vec![None, Some(Input::Char('q'))]);

        play(screen, keyboard).await.expect("a clean exit");

        assert_eq!(recording.drawn.lock().expect("the draw log").len(), 2);
    }

    #[tokio::test]
    async fn an_arrow_key_moves_like_its_vi_letter() {
        let (screen, recording) = recording_screen();
        let keyboard = scripted_keyboard(vec![Some(Input::Down), Some(Input::Char('q'))]);

        play(screen, keyboard).await.expect("a clean exit");

        let drawn = recording.drawn.lock().expect("the draw log");

        assert_eq!(drawn[1].0.grid.find(PLAYER), Some((0, 1)));
    }

    #[tokio::test]
    async fn enter_opens_a_prompt_and_a_second_enter_hands_the_line_to_the_controller() {
        let (screen, recording) = recording_screen();
        let keyboard = scripted_keyboard(vec![
            Some(Input::Enter),
            Some(Input::Char('h')),
            Some(Input::Char('i')),
            Some(Input::Enter),
            Some(Input::Char('q')),
        ]);

        play(screen, keyboard).await.expect("a clean exit");

        let drawn = recording.drawn.lock().expect("the draw log");

        assert_eq!(drawn[1].1, Prompt::Open(String::new()));
        assert_eq!(drawn[2].1, Prompt::Open("h".to_string()));
        assert_eq!(drawn[3].1, Prompt::Open("hi".to_string()));
        assert_eq!(drawn[4].1, Prompt::Closed);
        assert_eq!(drawn[4].0.message, "hi");
        assert_eq!(drawn[4].0.grid.find(PLAYER), Some((0, 0)));
    }

    #[tokio::test]
    async fn escape_closes_the_prompt_and_backspace_erases_one_character() {
        let (screen, recording) = recording_screen();
        let keyboard = scripted_keyboard(vec![
            Some(Input::Enter),
            Some(Input::Char('a')),
            Some(Input::Backspace),
            Some(Input::Escape),
            Some(Input::Char('q')),
        ]);

        play(screen, keyboard).await.expect("a clean exit");

        let drawn = recording.drawn.lock().expect("the draw log");

        assert_eq!(drawn[2].1, Prompt::Open("a".to_string()));
        assert_eq!(drawn[3].1, Prompt::Open(String::new()));
        assert_eq!(drawn[4].1, Prompt::Closed);
        assert_eq!(drawn[4].0.message, "");
    }

    #[tokio::test]
    async fn an_unbound_key_draws_nothing() {
        let (screen, recording) = recording_screen();
        let keyboard = scripted_keyboard(vec![Some(Input::Char('z')), Some(Input::Char('q'))]);

        play(screen, keyboard).await.expect("a clean exit");

        assert_eq!(recording.drawn.lock().expect("the draw log").len(), 1);
    }

    #[tokio::test]
    async fn control_c_stops_the_loop_and_leaves_the_screen() {
        let (screen, recording) = recording_screen();
        let keyboard = scripted_keyboard(vec![Some(Input::Interrupt)]);

        play(screen, keyboard).await.expect("a clean exit");

        assert_eq!(*recording.left.lock().expect("the leave count"), 1);
    }

    #[tokio::test]
    async fn a_keyboard_error_leaves_the_screen_and_comes_back_as_a_read_error() {
        let (screen, recording) = recording_screen();

        let mut keyboard = MockKeyboard::new();
        keyboard.expect_read().returning(|_| {
            Err(KeyboardError::Read {
                timeout_ms: 100,
                source: "the terminal went away".into(),
            })
        });

        let error = play(screen, keyboard).await.expect_err("a read error");

        assert!(matches!(error, TuiDriverError::Read { tick_ms: 100, .. }));
        assert_eq!(*recording.left.lock().expect("the leave count"), 1);
    }

    #[tokio::test]
    async fn a_screen_that_refuses_to_open_fails_bind_with_the_enter_error() {
        let mut screen = MockScreen::new();
        screen.expect_enter().returning(|| {
            Err(ScreenError::Enter {
                source: "no tty".into(),
            })
        });

        let error = play(screen, MockKeyboard::new())
            .await
            .expect_err("an enter error");

        assert!(matches!(error, TuiDriverError::Enter { .. }));
    }

    #[tokio::test]
    async fn serving_before_bind_is_refused_by_name() {
        let controller: Arc<dyn BoardController + Send + Sync> = Arc::new(controller());
        let driver = TuiDriver::new(TuiDriverConfig::default(), controller);

        let announced = driver.announce().expect_err("a refusal");
        let served = driver.serve().await.expect_err("a refusal");

        assert_eq!(
            announced.to_string(),
            "using the tui driver: it is not bound yet"
        );
        assert_eq!(
            served.to_string(),
            "using the tui driver: it is not bound yet"
        );
    }

    #[test]
    fn a_panic_inside_the_loop_still_leaves_the_screen() {
        let (screen, recording) = recording_screen();

        let mut keyboard = MockKeyboard::new();
        keyboard
            .expect_read()
            .returning(|_| panic!("the keyboard exploded"));

        let runtime = tokio::runtime::Builder::new_current_thread()
            .build()
            .expect("a runtime");

        let outcome = std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| {
            runtime.block_on(play(screen, keyboard))
        }));

        assert!(outcome.is_err());
        assert_eq!(*recording.left.lock().expect("the leave count"), 1);
    }
}
