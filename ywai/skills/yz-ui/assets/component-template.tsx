/**
 * Template de Componente React con Estilo Yoizen
 * 
 * Copiar este archivo para crear nuevos componentes consistentes
 * con el design system de Yoizen.
 */

import React from 'react';

// Tipos de propiedades del componente
interface ComponentNameProps {
  /** Título o contenido principal */
  title: string;
  /** Descripción opcional */
  description?: string;
  /** Variante visual */
  variant?: 'default' | 'primary' | 'secondary' | 'accent';
  /** Tamaño del componente */
  size?: 'sm' | 'md' | 'lg';
  /** Estado deshabilitado */
  disabled?: boolean;
  /** Handler de click */
  onClick?: () => void;
  /** Clases adicionales de Tailwind */
  className?: string;
  /** Contenido hijo */
  children?: React.ReactNode;
}

/**
 * ComponentName - Breve descripción del componente
 * 
 * @example
 * ```tsx
 * <ComponentName
 *   title="Título Ejemplo"
 *   description="Descripción opcional"
 *   variant="primary"
 *   size="md"
 *   onClick={() => console.log('clicked')}
 * />
 * ```
 */
export const ComponentName: React.FC<ComponentNameProps> = ({
  title,
  description,
  variant = 'default',
  size = 'md',
  disabled = false,
  onClick,
  className = '',
  children,
}) => {
  // Mapeo de variantes a clases
  const variantClasses = {
    default: 'bg-surface border-subtle text-primary',
    primary: 'bg-primary border-primary text-white',
    secondary: 'bg-surface border-primary/50 text-primary',
    accent: 'bg-accent/10 border-accent text-primary',
  };

  // Mapeo de tamaños
  const sizeClasses = {
    sm: 'p-2 text-sm',
    md: 'p-4 text-base',
    lg: 'p-6 text-lg',
  };

  // Estado deshabilitado
  const disabledClasses = disabled
    ? 'opacity-50 cursor-not-allowed'
    : 'cursor-pointer hover:opacity-90 transition-opacity';

  // Combinación de clases
  const classes = `
    rounded-lg border
    ${variantClasses[variant]}
    ${sizeClasses[size]}
    ${disabledClasses}
    ${className}
  `.trim();

  return (
    <div
      className={classes}
      onClick={!disabled ? onClick : undefined}
      role={onClick ? 'button' : undefined}
      tabIndex={onClick ? 0 : undefined}
    >
      {/* Header */}
      <h3 className="font-semibold text-primary mb-1">{title}</h3>
      
      {/* Descripción opcional */}
      {description && (
        <p className="text-sm text-secondary">{description}</p>
      )}
      
      {/* Contenido hijo */}
      {children && (
        <div className="mt-3">{children}</div>
      )}
    </div>
  );
};

export default ComponentName;
