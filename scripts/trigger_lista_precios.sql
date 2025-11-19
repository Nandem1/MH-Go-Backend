-- Trigger para notificar cuando se actualiza lista_precios_cantera
-- Este trigger puede ser usado con PostgreSQL LISTEN/NOTIFY para invalidar cache automáticamente

-- Función que notifica cuando se actualiza lista_precios_cantera
CREATE OR REPLACE FUNCTION notify_lista_precios_update()
RETURNS TRIGGER AS $$
BEGIN
    -- Notificar el cambio con el código_tivendo afectado
    PERFORM pg_notify('lista_precios_updated', NEW.codigo_tivendo::text);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger para INSERT y UPDATE
CREATE TRIGGER trigger_lista_precios_update
    AFTER INSERT OR UPDATE ON lista_precios_cantera
    FOR EACH ROW
    EXECUTE FUNCTION notify_lista_precios_update();

-- Trigger para DELETE (opcional, si también quieres invalidar cuando se elimina)
CREATE OR REPLACE FUNCTION notify_lista_precios_delete()
RETURNS TRIGGER AS $$
BEGIN
    PERFORM pg_notify('lista_precios_deleted', OLD.codigo_tivendo::text);
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_lista_precios_delete
    AFTER DELETE ON lista_precios_cantera
    FOR EACH ROW
    EXECUTE FUNCTION notify_lista_precios_delete();

-- Nota: Para usar estos triggers con el backend Go, necesitarías implementar
-- un listener de PostgreSQL LISTEN/NOTIFY en el código Go que escuche estos eventos
-- y llame a los endpoints de invalidación automáticamente.

